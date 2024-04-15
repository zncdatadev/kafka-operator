package container

import (
	"fmt"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type KafkaContainerBuilder struct {
	common.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *kafkav1alpha1.ResourcesSpec
	sslSpec                 *kafkav1alpha1.SslSpec
}

func NewKafkaContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *kafkav1alpha1.ResourcesSpec,
	sslSpec *kafkav1alpha1.SslSpec,
) *KafkaContainerBuilder {
	return &KafkaContainerBuilder{
		ContainerBuilder:        *common.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		sslSpec:                 sslSpec,
	}
}

func (d *KafkaContainerBuilder) ContainerName() string {
	return string(Kafka)
}

func (d *KafkaContainerBuilder) ContainerEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
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
					Key: common.ZookeeperDiscoveryKey,
				},
			},
		},
	}
}

func (d *KafkaContainerBuilder) ResourceRequirements() corev1.ResourceRequirements {
	return *util.ConvertToResourceRequirements(d.resourceSpec)
}

func (d *KafkaContainerBuilder) VolumeMount() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      DataVolumeName(),
			MountPath: DataMountPath,
		},
		{
			Name:      ConfigVolumeName(),
			MountPath: ConfigMountPath,
		},
		{
			Name:      Log4jVolumeName(),
			MountPath: Log4jMountPath,
		},
		{
			Name:      Log4jLogVolumeName(),
			MountPath: LogMountPath,
		},
	}
	if sslEnabled(d.sslSpec) {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TlsKeystoreInternalVolumeName(),
			MountPath: TlsKeystoreMountPath,
		})
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
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkav1alpha1.KafkaPortName)},
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
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkav1alpha1.KafkaPortName)},
		},
	}
}

// ContainerPorts  make container ports of data node
func (d *KafkaContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          kafkav1alpha1.KafkaPortName,
			ContainerPort: kafkav1alpha1.KafkaClientPort,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          kafkav1alpha1.InternalPortName,
			ContainerPort: kafkav1alpha1.InternalPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// CommandArgs command args
// ex: export NODE_PORT=$(cat /zncdata/tmp/kafka_nodeport)
func (d *KafkaContainerBuilder) CommandArgs() []string {
	return []string{fmt.Sprintf("export %s=$(cat %s/%s)", EnvNodePort, NodePortMountPath, NodePortFileName)}
}
