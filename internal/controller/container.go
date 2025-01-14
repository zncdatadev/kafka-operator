package controller

import (
	"fmt"
	"strings"

	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	opgputil "github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

// ContainerComponent use for define container name
type ContainerComponent string

type KafkaContainerBuilder struct {
	resourceSpec *commonsv1alpha1.ResourcesSpec

	zookeeperDiscoveryZNode string
	*security.KafkaTlsSecurity
	namespace    string
	groupSvcName string
}

func NewKafkaContainer(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	tlsSecurity *security.KafkaTlsSecurity,
	namespace string,
	groupSvcName string,
) *KafkaContainerBuilder {
	return &KafkaContainerBuilder{
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		KafkaTlsSecurity:        tlsSecurity,
		namespace:               namespace,
		groupSvcName:            groupSvcName,
	}
}

func (d *KafkaContainerBuilder) ContainerName() string {
	return string(Kafka)
}

func (d *KafkaContainerBuilder) ContainerEnv() []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name: EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: EnvNode,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: EnvZookeeperConnections,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: d.zookeeperDiscoveryZNode,
					},
					Key: ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  EnvKafkaLog4jOpts,
			Value: fmt.Sprintf("-Dlog4j.configuration=file:%s/%s", kafkav1alpha1.KubedoopLogConfigDir, kafkav1alpha1.Log4jFileName),
		},
		{
			Name: EnvJvmArgs,
			Value: fmt.Sprintf("-Djava.security.properties=%s/security.properties -javaagent:%s/jmx/jmx_prometheus_javaagent.jar=%d:%s/jmx/config.yaml",
				kafkav1alpha1.KubedoopConfigDir, kafkav1alpha1.KubedoopRoot, kafkav1alpha1.MetricsPort, kafkav1alpha1.KubedoopRoot),
		},
	}

	if d.resourceSpec != nil && d.resourceSpec.Memory != nil {
		memoryLimit := d.resourceSpec.Memory.Limit
		heap := fmt.Sprintf("-Xmx%dm", int(util.QuantityToMB(memoryLimit)*0.8))
		envs = append(envs, corev1.EnvVar{
			Name:  EnvKafkaHeapOpts,
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
		{
			Name:      kafkav1alpha1.KubedoopListenerBroker,
			MountPath: kafkav1alpha1.KubedoopListenerBrokerDir,
		},
		{
			Name:      kafkav1alpha1.KubedoopListenerBootstrap,
			MountPath: kafkav1alpha1.KubedoopListenerBootstrapDir,
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
	return KafkaContainerPorts(d.KafkaTlsSecurity)
}

func (d *KafkaContainerBuilder) Command() []string {
	return []string{"sh", "-c"}
}

// CommandArgs command args
// ex: export NODE_PORT=$(cat /kubedoop/tmp/kafka_nodepor
func (d *KafkaContainerBuilder) CommandArgs() []string {
	listenerConfig, err := GetKafkaListenerConfig(d.namespace, d.KafkaTlsSecurity, d.groupSvcName)
	if err != nil {
		return nil
	}
	listeners := listenerConfig.ListenersString()
	advertisedListers := listenerConfig.AdvertisedListenersString()
	lisenerSecurityProtocolMap := listenerConfig.ListenerSecurityProtocolMapString()

	var args []string
	// trap functions
	args = append(args, "set -x")
	// args = append(args, "while true; do sleep 1000; done")
	args = append(args, opgputil.CommonBashTrapFunctions)
	// remove vector shut down file command
	args = append(args, opgputil.RemoveVectorShutdownFileCommand())
	// kafka execute command
	args = append(args, "prepare_signal_handlers")
	args = append(args, d.LaunchCommand(listeners, advertisedListers, lisenerSecurityProtocolMap))
	args = append(args, "wait_for_termination")
	// create vector shut down file command
	args = append(args, opgputil.CreateVectorShutdownFileCommand())

	return []string{strings.Join(args, "\n")}
}

// kafka launch command
func (d *KafkaContainerBuilder) LaunchCommand(listeners, advertisedListers, lisenerSecurityProtocolMap string) string {
	return fmt.Sprintf(`bin/kafka-server-start.sh %s/%s --override "zookeeper.connect=${ZOOKEEPER}" --override "listeners=%s" --override "advertised.listeners=%s" --override "listener.security.protocol.map=%s" &`,
		kafkav1alpha1.KubedoopConfigDir, kafkav1alpha1.ServerFileName, listeners, advertisedListers, lisenerSecurityProtocolMap)
}
