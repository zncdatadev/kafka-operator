package controller

import (
	"context"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/controller/container"
	"github.com/zncdata-labs/kafka-operator/internal/controller/svc"
	"github.com/zncdata-labs/kafka-operator/internal/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var _ common.StatefulSetResourceType = &StatefulSetReconciler{}

type StatefulSetReconciler struct {
	common.WorkloadStyleUncheckedReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec]
}

func NewStatefulSet(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	replicate int32,
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
	return builder.Build(), nil
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
	sslSpec := s.MergedCfg.Config.Ssl
	zNode := s.Instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode
	imageName := util.ImageRepository(imageSpec.Repository, imageSpec.Tag)
	groupSvcName := svc.CreateGroupServiceName(s.Instance.GetName(), s.GroupName)
	svcHost := common.CreateDomainHost(groupSvcName, s.Instance.GetNamespace(), s.Instance.Spec.ClusterConfigSpec.ClusterDomain)
	builder := container.NewKafkaContainerBuilder(imageName, imageSpec.PullPolicy, zNode, resourceSpec, sslSpec, svcHost)
	kafkaContainer := builder.Build(builder)
	return []corev1.Container{
		kafkaContainer,
	}
}

// make init containers
func (s *StatefulSetReconciler) makeInitContainers() []corev1.Container {
	builder := container.NewFetchNodePortContainerBuilder()
	return []corev1.Container{
		builder.Build(builder),
	}
}

// make volumes
func (s *StatefulSetReconciler) volumes() []common.VolumeSpec {
	volumes := []common.VolumeSpec{
		{
			Name:       container.NodePortVolumeName(),
			SourceType: common.EmptyDir,
			Params:     &common.VolumeSourceParams{},
		},
		{
			Name:       container.Log4jLoggingVolumeName(),
			SourceType: common.EmptyDir,
			Params: &common.VolumeSourceParams{
				EmptyVolumeLimit: "40Mi",
			},
		},
		//{
		//	Name:       container.ConfigmapVolumeName(),
		//	SourceType: common.ConfigMap,
		//	Params: &common.VolumeSourceParams{
		//		ConfigMap: common.ConfigMapSpec{
		//			Name: common.CreateConfigName(s.Instance.GetName(), s.GroupName),
		//			KeyPath: []corev1.KeyToPath{
		//				{Key: kafkav1alpha1.ServerFileName, Path: "server.properties"},
		//			},
		//		}},
		//},
		{
			Name:       container.Log4jVolumeName(),
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
			Name:       container.ServerConfigVolumeName(),
			SourceType: common.EmptyDir,
		},
	}
	if common.SslEnabled(s.MergedCfg.Config.Ssl) {
		volumes = append(volumes, common.VolumeSpec{
			Name:       common.TlsKeystoreInternalVolumeName(),
			SourceType: common.EphemeralSecret,
			Params: &common.VolumeSourceParams{
				EphemeralSecret: &common.EphemeralSecretSpec{
					Annotations: s.tlsKeystoreAnnotations(), // !!!importance-1
					PvcSpec: common.PvcSpec{
						StorageClass: func() *string { s := common.SecretStorageClass; return &s }(), // !!!importance-2
						AccessModes:  []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageSize:  "1",
					},
				},
			},
		})
	}
	return volumes
}

// tls keystore annotations
// todo: cr define
func (s *StatefulSetReconciler) tlsKeystoreAnnotations() map[string]string {
	return map[string]string{
		common.SecretAnnotationClass:          string(common.Tls),
		common.SecretAnnotationFormat:         string(common.Pkcs12),
		common.SecretAnnotationScope:          strings.Join([]string{string(common.ScopePod), string(common.ScopeNode)}, ","),
		common.SecretAnnotationPKCS12Password: s.MergedCfg.Config.Ssl.KeyStorePassword,
	}
}

func (s *StatefulSetReconciler) pvcTemplates() []common.VolumeClaimTemplateSpec {
	return []common.VolumeClaimTemplateSpec{
		{
			Name: container.DataVolumeName(),
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
