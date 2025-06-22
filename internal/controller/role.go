package controller

import (
	"context"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
)

var logger = ctrl.Log.WithName("role-reconciler")

func NewBrokerReconciler(
	client *client.Client,
	roleInfo reconciler.RoleInfo,
	spec *kafkav1alpha1.BrokersSpec,
	image *opgoutil.Image,
	clusterConfig *kafkav1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	kafkaTlsSecurity *security.KafkaTlsSecurity,
) *BrokerReconciler {

	stopped := clusterOperation != nil && clusterOperation.Stopped

	return &BrokerReconciler{
		BaseRoleReconciler: *reconciler.NewBaseRoleReconciler(
			client,
			stopped,
			roleInfo,
			spec,
		),
		image:            image,
		clusterConfig:    clusterConfig,
		clusterOperation: clusterOperation,
		kafkaTlsSecurity: kafkaTlsSecurity,
	}
}

type BrokerReconciler struct {
	reconciler.BaseRoleReconciler[*kafkav1alpha1.BrokersSpec]

	clusterConfig    *kafkav1alpha1.ClusterConfigSpec
	clusterOperation *commonsv1alpha1.ClusterOperationSpec
	image            *opgoutil.Image
	kafkaTlsSecurity *security.KafkaTlsSecurity
}

func (r *BrokerReconciler) RegisterResources(ctx context.Context) error {
	for name, roleGroup := range r.Spec.RoleGroups {
		mergedConfig, err := opgoutil.MergeObject(r.Spec.Config, roleGroup.Config)
		if err != nil {
			return err
		}
		overrides, err := opgoutil.MergeObject(r.Spec.OverridesSpec, roleGroup.OverridesSpec)
		if err != nil {
			return err
		}

		// merge default config to the user provided config
		if overrides == nil {
			overrides = &commonsv1alpha1.OverridesSpec{}
		}
		if mergedConfig == nil {
			mergedConfig = &kafkav1alpha1.BrokersConfigSpec{}
		}
		err = MergeFromUserConfig(mergedConfig, overrides, r.GetClusterName())
		if err != nil {
			return err
		}

		info := &reconciler.RoleGroupInfo{
			RoleInfo:      r.RoleInfo,
			RoleGroupName: name,
		}
		reconcilers, err := r.RegisterResourceWithRoleGroup(
			ctx,
			roleGroup.Replicas,
			info,
			overrides,
			mergedConfig,
		)
		if err != nil {
			return err
		}

		for _, reconciler := range reconcilers {
			r.AddResource(reconciler)
			logger.Info("registered resource", "role", r.GetName(), "roleGroup", name, "reconciler", reconciler.GetName())
		}
	}
	return nil
}

func (r *BrokerReconciler) RegisterResourceWithRoleGroup(
	ctx context.Context,
	replicas int32,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	brokerConfig *kafkav1alpha1.BrokersConfigSpec,
) ([]reconciler.Reconciler, error) {

	var reconcilers = make([]reconciler.Reconciler, 0)

	// svc
	svc := NewRoleGroupService(r.Client, roleGroupInfo)
	reconcilers = append(reconcilers, svc)

	// configmap
	cm := NewKafkaConfigmapReconciler(
		ctx,
		r.Client,
		r.clusterConfig,
		r.kafkaTlsSecurity,
		roleGroupInfo,
		overrides,
		brokerConfig.RoleGroupConfigSpec,
	)
	reconcilers = append(reconcilers, cm)

	// statefulset
	sts := NewStatefulSetReconciler(
		ctx,
		r.Client,
		r.image,
		&replicas,
		r.clusterConfig,
		r.clusterOperation,
		roleGroupInfo,
		brokerConfig,
		overrides,
		r.kafkaTlsSecurity,
	)
	reconcilers = append(reconcilers, sts)

	// role group listener
	listener := NewRoleGroupBootstrapListenerReconciler(
		r.Client,
		brokerConfig.BootstrapListenerClass,
		roleGroupInfo,
		r.kafkaTlsSecurity,
	)
	reconcilers = append(reconcilers, listener)

	return reconcilers, nil
}
