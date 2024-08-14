package controller

import (
	"context"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	"github.com/zncdatadev/kafka-operator/internal/controller/container"
	"github.com/zncdatadev/kafka-operator/internal/controller/svc"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ common.StatefulSetResourceType = &StatefulSetReconciler{}

type StatefulSetReconciler struct {
	common.WorkloadStyleUncheckedReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec]
	*security.KafkaTlsSecurity
}

func NewStatefulSet(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	replicate int32,
	tlsSecurity *security.KafkaTlsSecurity,
) *StatefulSetReconciler {
	return &StatefulSetReconciler{
		WorkloadStyleUncheckedReconciler: *common.NewWorkloadStyleUncheckedReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
			replicate,
		),
		KafkaTlsSecurity: tlsSecurity,
	}
}

func (s *StatefulSetReconciler) Build(_ context.Context) (client.Object, error) {
	builder := common.NewStatefulSetBuilder(
		createStatefulSetName(s.Instance.GetName(), s.GroupName),
		s.Instance.Namespace,
		s.Labels,
		s.Replicas,
		svc.CreateGroupServiceName(s.Instance.GetName(), s.GroupName),
		s.makeKafkaContainer(),
	)
	builder.SetServiceAccountName(common.CreateServiceAccountName(s.Instance.GetName()))
	builder.SetVolumes(s.volumes())
	builder.SetPvcTemplates(s.pvcTemplates())
	builder.SetInitContainers(s.makeInitContainers())

	sts := builder.Build()
	// for security decor
	s.KafkaTlsSecurity.AddVolumeAndVolumeMounts(sts)

	return sts, nil
}

func (s *StatefulSetReconciler) SetAffinity(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	if affinity := s.MergedCfg.Config.Affinity; affinity != nil {
		dep.Spec.Template.Spec.Affinity = affinity
	} else {
		dep.Spec.Template.Spec.Affinity = common.AffinityDefault(common.Broker, s.Instance.GetName())
	}
}

func (s *StatefulSetReconciler) CommandOverride(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if cmdOverride := s.MergedCfg.CommandArgsOverrides; cmdOverride != nil {
		for i := range containers {
			if containers[i].Name == string(common.Kafka) {
				containers[i].Command = cmdOverride
				break
			}
		}
	}
}

func (s *StatefulSetReconciler) EnvOverride(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if envOverride := s.MergedCfg.EnvOverrides; envOverride != nil {
		for i := range containers {
			if containers[i].Name == string(common.Kafka) {
				envVars := containers[i].Env
				common.OverrideEnvVars(&envVars, s.MergedCfg.EnvOverrides)
				break
			}
		}
	}
}

func (s *StatefulSetReconciler) LogOverride(_ client.Object) {
	// do nothing, see name node
}

// make name node container
func (s *StatefulSetReconciler) makeKafkaContainer() []corev1.Container {
	imageSpec := s.Instance.Spec.Image
	resourceSpec := s.MergedCfg.Config.Resources
	zNode := s.Instance.Spec.ClusterConfig.ZookeeperConfigMapName
	imageName := util.ImageRepository(imageSpec)
	groupSvcName := svc.CreateGroupServiceName(s.Instance.GetName(), s.GroupName)
	builder := container.NewKafkaContainerBuilder(imageName, util.ImagePullPolicy(imageSpec), zNode, resourceSpec, s.KafkaTlsSecurity, s.Instance.Namespace, groupSvcName)
	kafkaContainer := builder.Build(builder)
	return []corev1.Container{
		kafkaContainer,
	}
}

// make init containers
func (s *StatefulSetReconciler) makeInitContainers() []corev1.Container {
	builder := container.NewFetchNodePortContainerBuilder(s.KafkaTlsSecurity)
	return []corev1.Container{
		builder.Build(builder),
	}
}

// make volumes
func (s *StatefulSetReconciler) volumes() []common.VolumeSpec {
	volumes := []common.VolumeSpec{
		{
			Name:       kafkav1alpha1.KubedoopTmpDirName,
			SourceType: common.EmptyDir,
			Params:     &common.VolumeSourceParams{},
		},
		{
			Name:       kafkav1alpha1.KubedoopLogDirName,
			SourceType: common.EmptyDir,
			Params: &common.VolumeSourceParams{
				EmptyVolumeLimit: "40Mi",
			},
		},
		{
			Name:       kafkav1alpha1.KubedoopLogConfigDirName,
			SourceType: common.ConfigMap,
			Params: &common.VolumeSourceParams{
				ConfigMap: common.ConfigMapSpec{
					Name: common.CreateConfigName(s.Instance.GetName(), s.GroupName),
					KeyPath: []corev1.KeyToPath{
						{Key: kafkav1alpha1.Log4jFileName, Path: "log4j.properties"},
					},
				}},
		},
		{
			Name:       kafkav1alpha1.KubedoopConfigDirName,
			SourceType: common.ConfigMap,
			Params: &common.VolumeSourceParams{
				ConfigMap: common.ConfigMapSpec{
					Name: common.CreateConfigName(s.Instance.GetName(), s.GroupName),
				},
			},
		},
	}
	return volumes
}

func (s *StatefulSetReconciler) pvcTemplates() []common.VolumeClaimTemplateSpec {
	return []common.VolumeClaimTemplateSpec{
		{
			Name: kafkav1alpha1.KubedoopKafkaDataDirName,
			PvcSpec: common.PvcSpec{

				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageSize: "2Gi",
			},
		},
	}
}

func (s *StatefulSetReconciler) GetConditions() *[]metav1.Condition {
	return &s.Instance.Status.Conditions
}
