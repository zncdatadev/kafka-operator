package svc

import (
	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewGroupServiceHeadless new a GroupServiceReconciler
func NewGroupServiceHeadless(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
) *common.GenericServiceReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec] {
	headlessType := common.HeadlessService
	buidler := common.NewServiceBuilder(
		CreateGroupServiceName(instance.GetName(), groupName),
		instance.GetNamespace(),
		labels,
		makeGroupSvcPorts(),
	).SetClusterIP(&headlessType)
	return common.NewGenericServiceReconciler(
		scheme,
		instance,
		client,
		groupName,
		labels,
		mergedCfg,
		buidler,
	)
}

func makeGroupSvcPorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       kafkav1alpha1.ClientPortName,
			Port:       GroupServiceClientPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.ClientPortName),
		},
		{
			Name:       kafkav1alpha1.InternalPortName,
			Port:       GroupServiceInternalPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.InternalPortName),
		},
	}
}
