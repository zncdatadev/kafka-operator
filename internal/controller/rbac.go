package controller

import (
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var serviceAccountName = func(instanceName string) string { return common.CreateServiceAccountName(instanceName) }
var roleName = "kafka-role"
var roleBindingName = "kafka-rolebinding"

// NewServiceAccount new a ServiceAccountReconciler
func NewServiceAccount(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg any,
) *common.GenericServiceAccountReconciler[*kafkav1alpha1.KafkaCluster, any] {
	return common.NewServiceAccount[*kafkav1alpha1.KafkaCluster](scheme, instance, client, mergedLabels, mergedCfg,
		serviceAccountName(instance.GetName()), instance.GetNamespace())
}

// NewRole new a ClusterRoleReconciler
func NewRole(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
) *common.GenericRoleReconciler[*kafkav1alpha1.KafkaCluster, any] {
	return common.NewRole[*kafkav1alpha1.KafkaCluster](
		scheme,
		instance,
		client,
		"",
		mergedLabels,
		mergedCfg,
		common.RbacRole,
		roleName,
		[]common.VerbType{common.Get, common.List, common.Watch},
		[]string{""},
		[]common.ResourceType{common.Services},
		instance.Namespace,
	)
}

// NewRoleBinding new a ClusterRoleBindingReconciler
func NewRoleBinding(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
) *common.GenericRoleBindingReconciler[*kafkav1alpha1.KafkaCluster, any] {
	return common.NewRoleBinding[*kafkav1alpha1.KafkaCluster](
		scheme,
		instance,
		client,
		"",
		mergedLabels,
		mergedCfg,

		"",
		common.RoleBinding,
		roleBindingName,
		roleName,
		serviceAccountName(instance.GetName()),
		instance.GetNamespace(),
	)
}
