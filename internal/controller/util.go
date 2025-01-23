package controller

import (
	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
)

func RoleGroupConfigMapName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName()
}

func ServiceAccountName(crName string) string {
	return crName
}

func BootstrapListenerName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName() + "-bootstrap"
}

func KafkaContainerPorts(kafkaTlsSecurity *security.KafkaTlsSecurity) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          kafkaTlsSecurity.ClientPortName(),
			ContainerPort: int32(kafkaTlsSecurity.ClientPort()),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          kafkav1alpha1.MetricsPortName,
			ContainerPort: kafkav1alpha1.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}
