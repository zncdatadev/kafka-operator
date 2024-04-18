package controller

import (
	"fmt"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDataNodeLogging(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	currentConfigMap *corev1.ConfigMap,
) *common.OverrideExistLoggingRecociler[*kafkav1alpha1.KafkaCluster, any] {
	logDataBuilder := LogDataBuilder{
		cfg:              mergedCfg,
		currentConfigMap: currentConfigMap,
	}
	return common.NewOverrideExistLoggingRecociler[*kafkav1alpha1.KafkaCluster](
		scheme,
		instance,
		client,
		groupName,
		mergedLabels,
		mergedCfg,
		&logDataBuilder,
	)
}

type LogDataBuilder struct {
	cfg              *kafkav1alpha1.BrokersRoleGroupSpec
	currentConfigMap *corev1.ConfigMap
}

func (l *LogDataBuilder) MakeContainerLogData() map[string]string {
	cmData := &l.currentConfigMap.Data
	if logging := l.cfg.Config.Logging; logging != nil {
		if kafka := logging.Broker; kafka != nil {
			l.OverrideConfigMapData(cmData, common.Kafka, kafka)
		}
	}
	return *cmData
}

// OverrideConfigMapData override log4j properties and update the configmap
func (l *LogDataBuilder) OverrideConfigMapData(cmData *map[string]string, container common.ContainerComponent,
	containerLogSpec *kafkav1alpha1.LoggingConfigSpec) {
	log4jBuilder := common.CreateLog4jBuilder(containerLogSpec, common.ConsoleLogAppender, common.FileLogAppender)
	log4jConfigMapKey := CreateComponentLog4jPropertiesName(container)
	override := log4jBuilder.MakeContainerLogProperties((*cmData)[log4jConfigMapKey])
	(*cmData)[log4jConfigMapKey] = override
}
func CreateComponentLog4jPropertiesName(component common.ContainerComponent) string {
	return fmt.Sprintf("%s.log4j.properties", string(component))
}
