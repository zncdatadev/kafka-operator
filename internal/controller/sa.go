package controller

import (
	"context"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccountReconciler struct {
	common.GeneralResourceStyleReconciler[*kafkav1alpha1.KafkaCluster, any]
}

// NewServiceAccount new a ServiceAccountReconciler
func NewServiceAccount(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,
) *ServiceAccountReconciler {
	return &ServiceAccountReconciler{
		GeneralResourceStyleReconciler: *common.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
	}
}

// Build implements the ResourceBuilder interface
func (r *ServiceAccountReconciler) Build(_ context.Context) (client.Object, error) {
	saToken := true
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.CreateServiceAccountName(r.Instance.GetName()),
			Namespace: r.Instance.Namespace,
			Labels:    r.MergedLabels,
		},
		AutomountServiceAccountToken: &saToken,
	}, nil
}
