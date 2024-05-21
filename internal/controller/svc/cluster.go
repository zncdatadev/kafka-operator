package svc

import (
	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
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
) *common.GenericServiceReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec] {
	headlessServiceType := common.Service
	serviceType := corev1.ServiceTypeNodePort
	builder := common.NewServiceBuilder(
		CreateGroupServiceName(instance.GetName(), ""),
		instance.GetNamespace(),
		labels,
		makePorts(),
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

func makePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       kafkav1alpha1.KafkaPortName,
			Port:       ClusterServiceClientPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.KafkaPortName),
			NodePort:   ClusterServiceClientNodePort,
		},
	}
}
