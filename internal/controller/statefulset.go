package controller

import (
	"context"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/controller/container"
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
		s.MergedLabels,
		s.Replicas,
		createServiceName(s.Instance.GetName(), s.GroupName),
		s.makeKafkaContainer(),
	)
	builder.SetServiceAccountName(common.CreateServiceAccountName(s.Instance.GetName()))
	builder.SetVolumes(s.volumes())
	builder.SetPvcTemplates(s.pvcTemplates())
	return builder.Build(), nil
}
func (s *StatefulSetReconciler) CommandOverride(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if cmdOverride := s.MergedCfg.CommandArgsOverrides; cmdOverride != nil {
		for i := range containers {
			if containers[i].Name == string(container.Kafka) {
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
			if containers[i].Name == string(container.Kafka) {
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
	return nil
}

// make volumes
func (s *StatefulSetReconciler) volumes() []common.VolumeSpec {
	return []common.VolumeSpec{
		{
			Name:       container.ConfigVolumeName(),
			SourceType: common.ConfigMap,
			Params: &common.VolumeSourceParams{
				ConfigMap: common.ConfigMapSpec{
					Name: common.CreateConfigName(s.Instance.GetName(), s.GroupName),
					KeyPath: []corev1.KeyToPath{
						{Key: kafkav1alpha1.ServerFileName, Path: "server.properties"},
					},
				}},
		},
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
			Name:       container.TlsKeystoreInternalVolumeName(),
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
		},
	}
}

// tls keystore annotations
func (s *StatefulSetReconciler) tlsKeystoreAnnotations() map[string]string {
	return map[string]string{
		common.SecretAnnotationClass:  string(common.Tls),
		common.SecretAnnotationFormat: string(common.Pkcs12),
		common.SecretAnnotationScope:  strings.Join([]string{string(common.ScopePod), string(common.ScopeNode)}, ","),
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
