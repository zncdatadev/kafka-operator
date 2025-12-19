package controller

import (
	"context"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
)

func NewStatefulSetReconciler(
	ctx context.Context,
	client *client.Client,
	image *opgoutil.Image,
	replicas *int32,
	clusterConfig *kafkav1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	roleGroupInf *reconciler.RoleGroupInfo,
	brokerConfig *kafkav1alpha1.BrokersConfigSpec,
	overrides *commonsv1alpha1.OverridesSpec,
	kafkaTlsSecurity *security.KafkaSecurity,
) reconciler.ResourceReconciler[builder.StatefulSetBuilder] {
	stopped := clusterOperation != nil && clusterOperation.Stopped

	builder := NewStatefulSetBuilder(
		ctx,
		client,
		image,
		replicas,
		clusterConfig,
		roleGroupInf,
		brokerConfig,
		overrides,
		kafkaTlsSecurity,
	)
	return reconciler.NewStatefulSet(client, builder, stopped)
}

func NewStatefulSetBuilder(
	ctx context.Context,
	client *client.Client,
	image *opgoutil.Image,
	replicas *int32,
	clusterConfig *kafkav1alpha1.ClusterConfigSpec,
	roleGroupInf *reconciler.RoleGroupInfo,
	brokerConfig *kafkav1alpha1.BrokersConfigSpec,
	overrdes *commonsv1alpha1.OverridesSpec,
	kafkaTlsSecurity *security.KafkaSecurity,
) builder.StatefulSetBuilder {

	return &StatefulSetBuilder{
		StatefulSet: *builder.NewStatefulSetBuilder(
			client,
			roleGroupInf.GetFullName(),
			replicas,
			image,
			overrdes,
			brokerConfig.RoleGroupConfigSpec,
			func(o *builder.Options) {
				o.ClusterName = roleGroupInf.ClusterName
				o.Labels = roleGroupInf.GetLabels()
				o.Annotations = roleGroupInf.GetAnnotations()
				o.RoleName = roleGroupInf.RoleName
				o.RoleGroupName = roleGroupInf.GetFullName()
			},
		),
		ClusterConfig:    clusterConfig,
		roleGroupInf:     roleGroupInf,
		brokerConfig:     brokerConfig,
		kafkaTlsSecurity: kafkaTlsSecurity,
	}
}

var _ builder.StatefulSetBuilder = &StatefulSetBuilder{}

type StatefulSetBuilder struct {
	builder.StatefulSet
	ClusterConfig    *kafkav1alpha1.ClusterConfigSpec
	roleGroupInf     *reconciler.RoleGroupInfo
	brokerConfig     *kafkav1alpha1.BrokersConfigSpec
	kafkaTlsSecurity *security.KafkaSecurity
}

func (b *StatefulSetBuilder) GetObject() (*appv1.StatefulSet, error) {
	tpl, err := b.GetPodTemplate()
	if err != nil {
		return nil, err
	}
	obj := &appv1.StatefulSet{
		ObjectMeta: b.GetObjectMeta(),
		Spec: appv1.StatefulSetSpec{
			Replicas:             b.GetReplicas(),
			Selector:             b.GetLabelSelector(),
			ServiceName:          b.GetName(),
			Template:             *tpl,
			VolumeClaimTemplates: b.GetVolumeClaimTemplates(),
		},
	}
	return obj, nil
}

func (b *StatefulSetBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {

	b.AddContainer(b.createMainContainer())
	bootstrapListenerPVC, err := b.bootstrapListenerPvc()
	if err != nil {
		return nil, err
	}
	b.AddVolumeClaimTemplates([]corev1.PersistentVolumeClaim{*bootstrapListenerPVC, *b.dataPvc()}) // data pvc, listener-bootstrap pvc

	volumes, err := b.Volumes(ctx)
	if err != nil {
		return nil, err
	}
	b.AddVolumes(volumes)

	roleGroupConfig := b.brokerConfig.RoleGroupConfigSpec
	// vector
	if IsVectorEnable(roleGroupConfig.Logging) {
		vectorFactory := GetVectorFactory(b.GetImage())
		b.AddContainer(vectorFactory.GetContainer())
		b.AddVolumes(vectorFactory.GetVolumes())
	}

	sts, err := b.GetObject()
	if err != nil {
		return nil, err
	}

	sts.Spec.Template.Spec.ServiceAccountName = ServiceAccountName(b.ClusterName) // TODO: add set service account name to builder
	// parallel pod management
	sts.Spec.PodManagementPolicy = appv1.ParallelPodManagement // TODO: add set pod management policy to builder.

	requestLifeTime := b.brokerConfig.RequestedSecretLifeTime
	b.kafkaTlsSecurity.AddVolumeAndVolumeMounts(sts, requestLifeTime)

	return sts, nil
}

func (b *StatefulSetBuilder) createMainContainer() *corev1.Container {
	image := b.GetImage()

	kafkaContainer := NewKafkaContainer(
		image.String(),
		image.GetPullPolicy(),
		b.ClusterConfig.ZookeeperConfigMapName,
		b.kafkaTlsSecurity,
		b.GetObjectMeta().Namespace,
		b.GetName(),
	)
	roleGroupConfig := b.brokerConfig.RoleGroupConfigSpec
	return builder.NewContainerBuilder(kafkaContainer.ContainerName(), image).
		AddEnvVars(kafkaContainer.ContainerEnv()).
		SetCommand(kafkaContainer.Command()).
		SetArgs(kafkaContainer.CommandArgs()).
		AddVolumeMounts(kafkaContainer.VolumeMount()).
		SetResources(roleGroupConfig.Resources).
		SetReadinessProbe(kafkaContainer.ReadinessProbe()).
		SetLivenessProbe(kafkaContainer.LivenessProbe()).
		AddPorts(kafkaContainer.ContainerPorts()).
		Build()
}

// Volumes
func (b *StatefulSetBuilder) Volumes(ctx context.Context) ([]corev1.Volume, error) {
	listenerVolumenSourceBuilder := util.NewListenerOperatorVolumeSourceBuilder(
		&util.ListenerReference{
			ListenerClass: b.brokerConfig.BrokerListenerClass,
		}, nil,
	)

	listenerPvc, err := listenerVolumenSourceBuilder.BuildEphemeral()
	if err != nil {
		return nil, err
	}

	volumes := []corev1.Volume{
		{
			Name: kafkav1alpha1.KubedoopLogDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: kafkav1alpha1.KubedoopLogConfigDirName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: RoleGroupConfigMapName(b.roleGroupInf),
					},
				},
			},
		},
		{
			Name: kafkav1alpha1.KubedoopConfigDirName,
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RoleGroupConfigMapName(b.roleGroupInf),
				},
			}},
		},
		// listener broker
		{
			Name: kafkav1alpha1.KubedoopListenerBroker,
			VolumeSource: corev1.VolumeSource{
				Ephemeral: listenerPvc,
			},
		},
	}

	if b.kafkaTlsSecurity.IsKerberosEnabled() {
		kerberosVolumes, err := b.kafkaTlsSecurity.KerberosAuth.GetVolumes(ctx)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, kerberosVolumes...)
	}
	return volumes, nil
}

// kafka log dirs pvc
func (b *StatefulSetBuilder) dataPvc() *corev1.PersistentVolumeClaim {
	capabilities := resource.MustParse("2Gi")
	var storageClassName *string
	roleGroupConfig := b.brokerConfig.RoleGroupConfigSpec
	dataStorage := roleGroupConfig.Resources.Storage
	if dataStorage != nil {
		capabilities = dataStorage.Capacity
		if dataStorage.StorageClass != "" {
			storageClassName = &dataStorage.StorageClass
		}
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kafkav1alpha1.KubedoopKafkaDataDirName,
			Namespace: b.GetObjectMeta().Namespace,
			Labels:    b.GetLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeMode:       ptr.To(corev1.PersistentVolumeFilesystem),
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: capabilities},
			},
		},
	}
}

// build bootstrap listener pvc
func (b *StatefulSetBuilder) bootstrapListenerPvc() (*corev1.PersistentVolumeClaim, error) {

	builder := util.NewListenerOperatorVolumeSourceBuilder(
		&util.ListenerReference{
			ListenerName: BootstrapListenerName(b.roleGroupInf),
		}, b.GetLabels(),
	)
	return builder.BuildPVC(kafkav1alpha1.KubedoopListenerBootstrap)
}
