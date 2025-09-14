package controller

import (
	"context"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util/version"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	resourceClient "github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"
)

var _ reconciler.Reconciler = &Reconciler{}

type Reconciler struct {
	reconciler.BaseCluster[*kafkav1alpha1.KafkaClusterSpec]
	ClusterConfig    *kafkav1alpha1.ClusterConfigSpec
	ClusterOperation *commonsv1alpha1.ClusterOperationSpec
}

func NewClusterReconciler(
	client *resourceClient.Client,
	clusterInfo reconciler.ClusterInfo,
	spec *kafkav1alpha1.KafkaClusterSpec,
) *Reconciler {

	return &Reconciler{
		BaseCluster: *reconciler.NewBaseCluster(
			client,
			clusterInfo,
			spec.ClusterOperation,
			spec,
		),
		ClusterConfig: spec.ClusterConfig,
	}

}

func (r *Reconciler) GetImage() *util.Image {
	image := util.NewImage(
		kafkav1alpha1.DefaultProductName,
		version.BuildVersion,
		kafkav1alpha1.DefaultProductVersion,
		func(options *util.ImageOptions) {
			options.Custom = r.Spec.Image.Custom
			options.Repo = r.Spec.Image.Repo
			options.PullPolicy = *r.Spec.Image.PullPolicy
		},
	)

	if r.Spec.Image.KubedoopVersion != "" {
		image.KubedoopVersion = r.Spec.Image.KubedoopVersion
	}
	if r.Spec.Image.ProductVersion != "" {
		image.ProductVersion = r.Spec.Image.ProductVersion
	}
	return image
}

func (r *Reconciler) RegisterResources(ctx context.Context) error {

	// RBAC
	sa := NewServiceAccountReconciler(r.Client, r.GetName())
	r.AddResource(sa)

	// role `Broker`
	roleInfo := reconciler.RoleInfo{ClusterInfo: r.ClusterInfo, RoleName: RoleName}

	cluster := r.Client.OwnerReference.(*kafkav1alpha1.KafkaCluster)
	tlsSecurity := security.NewKafkaSecurity(cluster)

	node := NewBrokerReconciler(
		r.Client,
		roleInfo,
		r.Spec.Brokers,
		r.GetImage(),
		r.ClusterConfig,
		r.ClusterOperation,
		tlsSecurity,
	)

	if err := node.RegisterResources(ctx); err != nil {
		return err
	}
	r.AddResource(node)

	// Discovery related resources:
	// Note: NodePort service is merged with the main service.
	// The access type is now controlled by listenerclass setting.
	// TODO: Consider deprecating separate NodePort handling
	discovery := NewKafkaDiscoveryReconciler(ctx, r.Client, tlsSecurity)
	nodePortDiscovery := NewKafkaDiscoveryNodePortReconciler(ctx, r.Client, tlsSecurity)
	r.AddResource(discovery)
	r.AddResource(nodePortDiscovery)

	return nil
}
