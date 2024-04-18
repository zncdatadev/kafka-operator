package controller

import (
	"context"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ common.ConfigMapType = &ConfigMapReconciler{}

type ConfigMapReconciler struct {
	common.ConfigurationStyleReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec]
}

// NewConfigMap new a ConfigMapReconciler
func NewConfigMap(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		ConfigurationStyleReconciler: *common.NewConfigurationStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
		),
	}
}
func (c *ConfigMapReconciler) Build(_ context.Context) (client.Object, error) {
	builder := common.ConfigMapBuilder{
		Name:      common.CreateConfigName(c.Instance.GetName(), c.GroupName),
		Namespace: c.Instance.Namespace,
		Labels:    c.Labels,
		ConfigGenerators: []common.ConfigGenerator{
			&common.Log4jConfGenerator{},
			&common.SecurityConfGenerator{},
			//&KafkaConfGenerator{sslSpec: c.MergedCfg.Config.Ssl},
		},
	}
	return builder.Build(), nil
}
func (c *ConfigMapReconciler) ConfigurationOverride(resource client.Object) {
	cm := resource.(*corev1.ConfigMap)
	overrides := c.MergedCfg.ConfigOverrides
	if overrides != nil {
		if log4j := overrides.Log4j; log4j != nil {
			overridden := common.OverrideConfigFileContent(cm.Data[kafkav1alpha1.Log4jFileName], log4j, common.Properties)
			cm.Data[kafkav1alpha1.Log4jFileName] = overridden
		}
		if security := overrides.Security; security != nil {
			overridden := common.OverrideConfigFileContent(cm.Data[kafkav1alpha1.SecurityFileName], security, common.Properties)
			cm.Data[kafkav1alpha1.SecurityFileName] = overridden
		}
		//if server := overrides.Server; server != nil {
		//	overridden := common.OverrideConfigFileContent(cm.Data[kafkav1alpha1.ServerFileName], server, common.Properties)
		//	cm.Data[kafkav1alpha1.ServerFileName] = overridden
		//}
	}
	c.LoggingOverride(cm)
}

func (c *ConfigMapReconciler) LoggingOverride(current *corev1.ConfigMap) {
	logging := NewKafkaLogging(c.Scheme, c.Instance, c.Client, c.GroupName, c.Labels, c.MergedCfg, current)
	logging.OverrideExist(current)
}
