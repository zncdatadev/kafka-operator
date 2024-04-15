package controller

import (
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var serviceAccountName = func(instanceName string) string { return common.CreateServiceAccountName(instanceName) }
var clusterRoleName = "kafka-clusterrole"
var clusterRoleBindingName = "kafka-clusterrolebinding"

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

// NewClusterRole new a ClusterRoleReconciler
func NewClusterRole(
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

		common.RbacClusterRole,
		clusterRoleName,
		[]common.VerbType{common.Get, common.List, common.Watch},
		nil,
		[]common.ResourceType{common.ServiceAccounts, common.ConfigMaps, common.Pods, common.Services, common.StatefulSets},
		instance.Namespace,
	)
}

// NewClusterRoleBinding new a ClusterRoleBindingReconciler
func NewClusterRoleBinding(
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
		common.ClusterRoleBinding,
		clusterRoleBindingName,
		clusterRoleName,
		serviceAccountName(instance.GetName()),
		instance.GetNamespace(),
	)
}
