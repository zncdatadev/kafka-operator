/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/util/retry"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

// KafkaClusterReconciler reconciles a KafkaCluster object
type KafkaClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=kafka.zncdata.dev,resources=kafkaclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.zncdata.dev,resources=kafkaclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.zncdata.dev,resources=kafkaclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KafkaCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *KafkaClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling Kafka cluster instance")

	cr := &kafkav1alpha1.KafkaCluster{}

	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "unable to fetch KafkaCluster")
			return ctrl.Result{}, err
		}
		r.Log.Info("Kafka-cluster resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	r.Log.Info("KafkaCluster found", "Name", cr.Name)
	// reconcile order by "cluster -> role -> role-group -> resource"
	result, err := NewClusterReconciler(r.Client, r.Scheme, cr).ReconcileCluster(ctx)
	if err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}
	r.Log.Info("Reconcile successfully ", "Name", cr.Name)
	return ctrl.Result{}, nil
}

// UpdateStatus updates the status of the KafkaCluster resource
// https://stackoverflow.com/questions/76388004/k8s-controller-update-status-and-condition
func (r *KafkaClusterReconciler) UpdateStatus(ctx context.Context, instance *kafkav1alpha1.KafkaCluster) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, instance)
		//return r.Status().Patch(ctx, instance, client.MergeFrom(instance))
	})

	if retryErr != nil {
		r.Log.Error(retryErr, "Failed to update vfm status after retries")
		return retryErr
	}

	r.Log.V(1).Info("Successfully patched object status")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1alpha1.KafkaCluster{}).
		Complete(r)
}
