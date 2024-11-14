package controller

import (
	"context"

	"github.com/go-logr/logr"
	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	svc2 "github.com/zncdatadev/kafka-operator/internal/controller/svc"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	cr     *kafkav1alpha1.KafkaCluster
	Log    logr.Logger

	roleReconcilers     []common.RoleReconciler
	resourceReconcilers []common.ResourceReconciler
	*security.KafkaTlsSecurity
}

func NewClusterReconciler(client client.Client, scheme *runtime.Scheme, cr *kafkav1alpha1.KafkaCluster) *ClusterReconciler {
	c := &ClusterReconciler{
		client: client,
		scheme: scheme,
		cr:     cr,
	}
	c.KafkaTlsSecurity = security.NewKafkaTlsSecurity(c.cr.Spec.ClusterConfig.Tls)
	c.RegisterRole()
	c.RegisterResource()
	return c
}

// RegisterRole register role reconciler
func (c *ClusterReconciler) RegisterRole() {
	c.roleReconcilers = []common.RoleReconciler{
		NewRoleBroker(c.scheme, c.cr, c.client, c.Log),
	}
}

func (c *ClusterReconciler) RegisterResource() {
	// registry sa resource
	labels := common.RoleLabels{
		InstanceName: c.cr.Name,
	}

	sa := NewServiceAccount(c.scheme, c.cr, c.client, labels.GetLabels(), nil)
	role := NewRole(c.scheme, c.cr, c.client, labels.GetLabels(), nil)
	roleBinding := NewRoleBinding(c.scheme, c.cr, c.client, labels.GetLabels(), nil)
	svc := svc2.NewClusterService(c.scheme, c.cr, c.client, labels.GetLabels(), nil, c.KafkaTlsSecurity)
	c.resourceReconcilers = []common.ResourceReconciler{sa, role, roleBinding, svc}
}

func (c *ClusterReconciler) ReconcileCluster(ctx context.Context) (ctrl.Result, error) {
	c.preReconcile()

	// reconcile resource of cluster level
	if len(c.resourceReconcilers) > 0 {
		res, err := common.ReconcilerDoHandler(ctx, c.resourceReconcilers)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	// reconcile role
	for _, r := range c.roleReconcilers {
		res, err := r.ReconcileRole(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	// reconcile discovery
	res, err := c.ReconcileDiscovery(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if res.RequeueAfter > 0 {
		return res, nil
	}

	return ctrl.Result{}, nil
}

func (c *ClusterReconciler) preReconcile() {
	// pre-reconcile
	// merge all the role-group cfg of roles, and cache it
	// because of existing role group config circle reference
	// we need to cache it before reconcile
	for _, r := range c.roleReconcilers {
		r.CacheRoleGroupConfig()
	}
}

func (c *ClusterReconciler) ReconcileDiscovery(ctx context.Context) (ctrl.Result, error) {
	discovery := NewDiscovery(c.scheme, c.cr, c.client, c.KafkaTlsSecurity)
	return discovery.ReconcileResource(ctx, common.NewSingleResourceBuilder(discovery))
}

type KafkaClusterInstance struct {
	Instance *kafkav1alpha1.KafkaCluster
}

func (h *KafkaClusterInstance) GetRoleConfigSpec(_ common.Role) (any, error) {
	return h.Instance.Spec.Brokers, nil
}

func (h *KafkaClusterInstance) GetClusterConfig() any {
	return h.Instance.Spec.ClusterConfig
}

func (h *KafkaClusterInstance) GetNamespace() string {
	return h.Instance.GetNamespace()
}

func (h *KafkaClusterInstance) GetInstanceName() string {
	return h.Instance.GetName()
}
