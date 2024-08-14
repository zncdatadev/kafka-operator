package controller

import (
	"context"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	"github.com/zncdatadev/kafka-operator/internal/controller/svc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const discoveryKey = "KAFKA"

//const nodeDiscoveryKey = "KAFKA_NODE"

type Discovery struct {
	common.GeneralResourceStyleReconciler[*kafkav1alpha1.KafkaCluster, any]
}

func NewDiscovery(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
) *Discovery {
	var mergedCfg any
	d := &Discovery{
		GeneralResourceStyleReconciler: *common.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			"",
			nil,
			mergedCfg,
		),
	}
	return d
}

// Build implements the ResourceBuilder interface
func (d *Discovery) Build(ctx context.Context) (client.Object, error) {
	clusterDomain := d.Instance.Spec.ClusterConfig.ClusterDomain
	clusterSvcName := svc.CreateClusterServiceName(d.Instance.GetName())
	dnsDomain := common.CreateDnsDomain(clusterSvcName, d.Instance.Namespace, clusterDomain, svc.ClusterServiceClientPort)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Instance.GetName(),
			Namespace: d.Instance.Namespace,
			Labels:    d.Labels,
		},
		Data: map[string]string{
			discoveryKey: dnsDomain,
			//nodeDiscoveryKey:
		},
	}, nil
}

// get nodes
