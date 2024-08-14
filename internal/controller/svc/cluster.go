package svc

import (
	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	"github.com/zncdatadev/kafka-operator/internal/security"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClusterService(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	tlsSecurity *security.KafkaTlsSecurity,
) *common.GenericServiceReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec] {
	headlessServiceType := common.Service
	serviceType := corev1.ServiceTypeNodePort
	builder := common.NewServiceBuilder(
		CreateGroupServiceName(instance.GetName(), ""),
		instance.GetNamespace(),
		labels,
		makePorts(tlsSecurity),
	).SetClusterIP(&headlessServiceType).SetType(&serviceType)
	return common.NewGenericServiceReconciler(
		scheme,
		instance,
		client,
		"",
		labels,
		mergedCfg,
		builder)
}

func makePorts(tlsSecurity *security.KafkaTlsSecurity) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       tlsSecurity.ClientPortName(),
			Port:       int32(tlsSecurity.ClientPort()),
			TargetPort: intstr.FromString(tlsSecurity.ClientPortName()),
		},
		{
			Name:       kafkav1alpha1.MetricsPortName,
			Port:       kafkav1alpha1.MetricsPort,
			TargetPort: intstr.FromString(kafkav1alpha1.MetricsPortName),
		},
	}
}
