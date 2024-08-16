package container

import (
	"fmt"
	"strings"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type KafkaContainerBuilder struct {
	common.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *kafkav1alpha1.ResourcesSpec
	*security.KafkaTlsSecurity
	namespace    string
	groupSvcName string
}

func NewKafkaContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *kafkav1alpha1.ResourcesSpec,
	tlsSecurity *security.KafkaTlsSecurity,
	namespace string,
	groupSvcName string,
) *KafkaContainerBuilder {
	return &KafkaContainerBuilder{
		ContainerBuilder:        *common.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		KafkaTlsSecurity:        tlsSecurity,
		namespace:               namespace,
		groupSvcName:            groupSvcName,
	}
}

func (d *KafkaContainerBuilder) ContainerName() string {
	return string(common.Kafka)
}

func (d *KafkaContainerBuilder) ContainerEnv() []corev1.EnvVar {
	// kafkaCfgGenerator := config.KafkaServerConfGenerator{
	// 	KafkaTlsSecurity: d.KafkaTlsSecurity,
	// }
	envs := []corev1.EnvVar{
		{
			Name: common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: common.EnvNode,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: common.EnvZookeeperConnections,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: d.zookeeperDiscoveryZNode,
					},
					Key: common.ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  common.EnvKafkaLog4jOpts,
			Value: fmt.Sprintf("-Dlog4j.configuration=file:%s/%s", kafkav1alpha1.KubedoopLogConfigDir, kafkav1alpha1.Log4jFileName),
		},
		{
			Name: common.EnvJvmArgs,
			Value: fmt.Sprintf("-Djava.security.properties=%s/security.properties -javaagent:%s/jmx/jmx_prometheus_javaagent.jar=%d:%s/jmx/config.yaml",
				kafkav1alpha1.KubedoopConfigDir, kafkav1alpha1.KubedoopRoot, kafkav1alpha1.MetricsPort, kafkav1alpha1.KubedoopRoot),
		},
	}

	if d.resourceSpec != nil && d.resourceSpec.Memory != nil && d.resourceSpec.Memory.Limit != nil {
		memoryLimit := *d.resourceSpec.Memory.Limit
		heap := fmt.Sprintf("-Xmx%dm", int(util.QuantityToMB(memoryLimit)*0.8))
		envs = append(envs, corev1.EnvVar{
			Name:  common.EnvKafkaHeapOpts,
			Value: heap,
		})
	}
	return envs
}

// create listener

func (d *KafkaContainerBuilder) ResourceRequirements() corev1.ResourceRequirements {
	return *util.ConvertToResourceRequirements(d.resourceSpec)
}

func (d *KafkaContainerBuilder) VolumeMount() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      kafkav1alpha1.KubedoopTmpDirName,
			MountPath: kafkav1alpha1.KubedoopTmpDir,
		},
		{
			Name:      kafkav1alpha1.KubedoopKafkaDataDirName,
			MountPath: kafkav1alpha1.KubedoopDataDir,
		},
		{
			Name:      kafkav1alpha1.KubedoopConfigDirName,
			MountPath: kafkav1alpha1.KubedoopConfigDir,
		},
		{
			Name:      kafkav1alpha1.KubedoopLogDirName,
			MountPath: kafkav1alpha1.KubedoopLogDir,
		},
		{
			Name:      kafkav1alpha1.KubedoopLogConfigDirName,
			MountPath: kafkav1alpha1.KubedoopLogConfigDir,
		},
	}
	return mounts
}

func (d *KafkaContainerBuilder) LivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    6,
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      5,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(d.KafkaTlsSecurity.ClientPortName())},
		},
	}
}

func (d *KafkaContainerBuilder) ReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(d.KafkaTlsSecurity.ClientPortName())},
		},
	}
}

// ContainerPorts  make container ports of data node
func (d *KafkaContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          d.KafkaTlsSecurity.ClientPortName(),
			ContainerPort: int32(d.KafkaTlsSecurity.ClientPort()),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          kafkav1alpha1.MetricsPortName,
			ContainerPort: kafkav1alpha1.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func (d *KafkaContainerBuilder) Command() []string {
	return []string{"sh", "-c"}
}

// CommandArgs command args
// ex: export NODE_PORT=$(cat /zncdata/tmp/kafka_nodepor
func (d *KafkaContainerBuilder) CommandArgs() []string {
	listenerConfig, err := common.GetKafkaListenerConfig(d.namespace, d.KafkaTlsSecurity, d.groupSvcName)
	if err != nil {
		return nil
	}
	listeners := listenerConfig.ListenersString()
	advertisedListers := listenerConfig.AdvertisedListenersString()
	lisenerSecurityProtocolMap := listenerConfig.ListenerSecurityProtocolMapString()

	var args []string
	// trap functions
	args = append(args, util.CommonBashTrapFunctions)
	// remove vector shut down file command
	args = append(args, util.RemoveVectorShutdownFileCommand(kafkav1alpha1.KubedoopLogDir))
	// kafka execute command
	args = append(args, "prepare_signal_handlers")
	args = append(args, d.LaunchCommand(listeners, advertisedListers, lisenerSecurityProtocolMap))
	args = append(args, "wait_for_termination")
	// create vector shut down file command
	args = append(args, util.CreateVectorShutdownFileCommand(kafkav1alpha1.KubedoopLogDir))

	return []string{strings.Join(args, "\n")}
}

// kafka launch command
func (d *KafkaContainerBuilder) LaunchCommand(listeners, advertisedListers, lisenerSecurityProtocolMap string) string {
	return fmt.Sprintf(`bin/kafka-server-start.sh %s/%s --override "zookeeper.connect=${ZOOKEEPER}" --override "listeners=%s" --override "advertised.listeners=%s" --override "listener.security.protocol.map=%s" &`,
		kafkav1alpha1.KubedoopConfigDir, kafkav1alpha1.ServerFileName, listeners, advertisedListers, lisenerSecurityProtocolMap)
}
