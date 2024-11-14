package common

import (
	"context"
	"fmt"
	"time"

	"github.com/zncdatadev/kafka-operator/internal/util"
	opgostatus "github.com/zncdatadev/operator-go/pkg/status"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceBuilderType union type for resource builder
// it will build the single resource or multi resources
type ResourceBuilderType struct {
	Single ResourceBuilder
	Multi  MultiResourceReconcilerBuilder
}

func NewSingleResourceBuilder(builder ResourceBuilder) ResourceBuilderType {
	return ResourceBuilderType{
		Single: builder,
	}
}

func NewMultiResourceBuilder(builder MultiResourceReconcilerBuilder) ResourceBuilderType {
	return ResourceBuilderType{
		Multi: builder,
	}
}

type ResourceReconciler interface {
	ReconcileResource(ctx context.Context, builder ResourceBuilderType) (ctrl.Result, error)
}

// ResourceValidator validate the resource is satisfied or completed
// such as workload's replicas, multi resources is created completely
type ResourceValidator interface {
	Validate(ctx context.Context) (ctrl.Result, error)
}

type ResourceBuilder interface {
	Build(ctx context.Context) (client.Object, error)
}

// MultiResourceReconcilerBuilder multi resource builder
// it will build multi resources
// for example, it will build more than one configMap
// currently, it is used to build the configMap
// see MultiResourceReconciler
type MultiResourceReconcilerBuilder interface {
	Build(ctx context.Context) ([]ResourceBuilder, error)
}

type ResourceHandler interface {
	DoReconcile(ctx context.Context, resource client.Object, instance ResourceHandler) (ctrl.Result, error)
}
type ConditionsGetter interface {
	GetConditions() *[]metav1.Condition
}

type AffinitySetter interface {
	SetAffinity(resource client.Object)
}

type WorkloadOverride interface {
	CommandOverride(resource client.Object)
	EnvOverride(resource client.Object)
	LogOverride(resource client.Object)
}

type ConfigurationOverride interface {
	ConfigurationOverride(resource client.Object)
}

type ResourceScope string

const (
	ScopeNameSpace ResourceScope = "namespace"
	ScopeCluster   ResourceScope = "cluster"
)

type BaseResourceReconciler[T client.Object, G any] struct {
	Instance  T
	Scheme    *runtime.Scheme
	Client    client.Client
	GroupName string

	Labels    map[string]string
	MergedCfg G
	owner     metav1.Object
	scope     *ResourceScope
}

// NewBaseResourceReconciler new a BaseResourceReconciler
func NewBaseResourceReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G) *BaseResourceReconciler[T, G] {
	return &BaseResourceReconciler[T, G]{
		Instance:  instance,
		Scheme:    scheme,
		Client:    client,
		GroupName: groupName,
		Labels:    mergedLabels,
		MergedCfg: mergedCfg,
	}
}

// SetOwner set owner
func (b *BaseResourceReconciler[T, G]) SetOwner(owner metav1.Object) *BaseResourceReconciler[T, G] {
	b.owner = owner
	return b
}

// SetScope set Scope
// cluster scoped should not set namespace, and also not set owner
func (b *BaseResourceReconciler[T, G]) SetScope(scope *ResourceScope) *BaseResourceReconciler[T, G] {
	b.scope = scope
	return b
}

func (b *BaseResourceReconciler[T, G]) ReconcileResource(
	ctx context.Context,
	builder ResourceBuilderType) (ctrl.Result, error) {
	// 1. mergelables
	// 2. build resource
	// 3. setControllerReference
	resInstance := builder.Single
	obj, err := resInstance.Build(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	// resInstance reconcile
	// return b.DoReconcile(ctx, resource)
	if handler, ok := resInstance.(ResourceHandler); ok {
		return handler.DoReconcile(ctx, obj, handler)
	} else {
		panic(fmt.Sprintf("resource is not ResourceHandler, actual is - %T", resInstance))
	}
}

func (b *BaseResourceReconciler[T, G]) Apply(
	ctx context.Context,
	dep client.Object,
	timeAfter time.Duration) (ctrl.Result, error) {
	if dep == nil {
		return ctrl.Result{}, nil
	}
	if b.scope != nil && *b.scope == ScopeCluster {
		// cluster scope need not set owner reference
		// cluster scoped should not set namespace, and also not set owner
	} else {
		if b.owner == nil {
			b.owner = b.Instance
		}
		if err := ctrl.SetControllerReference(b.owner, dep, b.Scheme); err != nil {
			return ctrl.Result{}, err
		}
	}

	mutant, err := util.CreateOrUpdate(ctx, b.Client, dep)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mutant {
		return ctrl.Result{RequeueAfter: timeAfter}, nil
	}
	return ctrl.Result{}, nil
}

//

// GeneralResourceStyleReconciler general style resource reconcile
// this reconciler is used to reconcile the general style resources
// such as configMap, secret, svc, etc.
type GeneralResourceStyleReconciler[T client.Object, G any] struct {
	BaseResourceReconciler[T, G]
}

func NewGeneraResourceStyleReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
) *GeneralResourceStyleReconciler[T, G] {
	return &GeneralResourceStyleReconciler[T, G]{
		BaseResourceReconciler: *NewBaseResourceReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
	}
}

func (s *GeneralResourceStyleReconciler[T, G]) DoReconcile(
	ctx context.Context,
	resource client.Object,
	_ ResourceHandler,
) (ctrl.Result, error) {
	return s.Apply(ctx, resource, time.Second*5)
}

// ConfigurationStyleReconciler configuration style reconciler
// this reconciler is used to reconcile the configuration style resources
// such as configMap, secret, etc.
// it will do the following things:
//  1. build resource
//  1. apply the resource
//
// Additional:
//  1. configuration override support
type ConfigurationStyleReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
}

func NewConfigurationStyleReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
) *ConfigurationStyleReconciler[T, G] {
	return &ConfigurationStyleReconciler[T, G]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
	}
}

func (s *ConfigurationStyleReconciler[T, G]) DoReconcile(
	ctx context.Context,
	resource client.Object,
	instance ResourceHandler,
) (ctrl.Result, error) {
	if resource == nil {
		return ctrl.Result{}, nil
	}
	if override, ok := instance.(ConfigurationOverride); ok {
		override.ConfigurationOverride(resource)
	} else {
		panic("resource is not ConfigurationOverride")
	}
	return s.Apply(ctx, resource, time.Second*5)
}

type WorkloadStyleUncheckedReconciler[T client.Object, G any] struct {
	BaseResourceReconciler[T, G]
	Replicas int32
}

func NewWorkloadStyleUncheckedReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	replicas int32,
) *WorkloadStyleUncheckedReconciler[T, G] {
	return &WorkloadStyleUncheckedReconciler[T, G]{
		BaseResourceReconciler: *NewBaseResourceReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
		Replicas: replicas,
	}
}

func (s *WorkloadStyleUncheckedReconciler[T, G]) DoReconcile(
	ctx context.Context,
	resource client.Object,
	instance ResourceHandler,
) (ctrl.Result, error) {
	// apply resource
	// check if the resource is satisfied
	// if not, return requeue
	// if satisfied, return nil
	if setAffinity, ok := instance.(AffinitySetter); ok {
		setAffinity.SetAffinity(resource)
	}
	if override, ok := instance.(WorkloadOverride); ok {
		override.CommandOverride(resource)
		override.EnvOverride(resource)
		override.LogOverride(resource)
	} else {
		panic("resource is not WorkloadOverride")
	}

	if res, err := s.Apply(ctx, resource, time.Second*10); err != nil {
		return ctrl.Result{}, err
	} else if res.RequeueAfter > 0 {
		return res, nil
	}

	// no check if the pods are satisfied
	return ctrl.Result{}, nil
}

// WorkloadStyleReconciler workload style reconciler
// this reconciler is used to reconcile the workload style resources
// such as workload, statefulSet, etc.
//
// it will do the following things:
//  0. build resource
//  1. apply the resource
//  2. check if the resource is satisfied
//  3. if not, return requeue
//  4. if satisfied, return nil
//
// Additional:
//  1. command and env override can support
//  2. logging override can support
type WorkloadStyleReconciler[T client.Object, G any] struct {
	BaseResourceReconciler[T, G]
	Replicas int32
}

func NewWorkloadStyleReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	replicas int32,
) *WorkloadStyleReconciler[T, G] {
	return &WorkloadStyleReconciler[T, G]{
		BaseResourceReconciler: *NewBaseResourceReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
		Replicas: replicas,
	}
}

func (s *WorkloadStyleReconciler[T, G]) DoReconcile(
	ctx context.Context,
	resource client.Object,
	instance ResourceHandler,
) (ctrl.Result, error) {
	// apply resource
	// check if the resource is satisfied
	// if not, return requeue
	// if satisfied, return nil
	if override, ok := instance.(WorkloadOverride); ok {
		override.CommandOverride(resource)
		override.EnvOverride(resource)
		override.LogOverride(resource)
	} else {
		panic("resource is not WorkloadOverride")
	}

	if res, err := s.Apply(ctx, resource, time.Second*10); err != nil {
		return ctrl.Result{}, err
	} else if res.RequeueAfter > 0 {
		return res, nil
	}

	// Check if the pods are satisfied
	return s.CheckPodsSatisfied(ctx, instance)
}

// CheckPodsSatisfied check if the pods are satisfied
func (s *WorkloadStyleReconciler[T, G]) CheckPodsSatisfied(ctx context.Context, handler ResourceHandler) (ctrl.Result, error) {
	// Check if the pods are satisfied
	satisfied, err := s.CheckPodReplicas(ctx)
	if err != nil {
		log.Error(err, "failed to check if the pods are satisfied", "labels", s.Labels)
		return ctrl.Result{}, err
	}

	if satisfied {
		err = s.updateStatus(
			metav1.ConditionTrue,
			"WorkloadSatisfied",
			"Workload is satisfied",
			handler,
		)
		if err != nil {
			log.Error(err, "failed to update status when workload is satisfied", "labels", s.Labels)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	err = s.updateStatus(
		metav1.ConditionFalse,
		"WorkloadNotSatisfied",
		"Workload is not satisfied",
		handler,
	)
	if err != nil {
		log.Error(err, "failed to update status when workload is not satisfied", "labels", s.Labels)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (s *WorkloadStyleReconciler[T, G]) CheckPodReplicas(ctx context.Context) (bool, error) {
	pods := corev1.PodList{}
	podListOptions := []client.ListOption{
		client.InNamespace(s.Instance.GetNamespace()),
		client.MatchingLabels(s.Labels),
	}
	err := s.Client.List(ctx, &pods, podListOptions...)
	if err != nil {
		log.Error(err, "failed to list pods")
		return false, err
	}
	return len(pods.Items) == int(s.Replicas), nil
}

func (s *WorkloadStyleReconciler[T, G]) updateStatus(
	status metav1.ConditionStatus,
	reason string,
	message string,
	instance ResourceHandler) error {
	if conditionHandler, ok := instance.(ConditionsGetter); ok {
		apimeta.SetStatusCondition(conditionHandler.GetConditions(), metav1.Condition{
			Type:               opgostatus.ConditionTypeAvailable,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: s.Instance.GetGeneration(),
		})
		// return s.Client.Status().Update(context.Background(), s.Instance) todo: for tests, some bugs to fix
		return nil
	} else {
		panic("instance is not ConditionsGetter")
	}
}

// MultiResourceReconciler multi configuration object reconciler
type MultiResourceReconciler[T client.Object, G any] struct {
	BaseResourceReconciler[T, G]
}

func NewMultiResourceReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
) *MultiResourceReconciler[T, G] {
	return &MultiResourceReconciler[T, G]{
		BaseResourceReconciler: *NewBaseResourceReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
	}
}

// ReconcileResource implement ResourceReconcile interface
func (s *MultiResourceReconciler[T, G]) ReconcileResource(
	ctx context.Context,
	builder ResourceBuilderType) (ctrl.Result, error) {
	// 1. mergelables
	// 2. build multi resource
	// 3. setControllerReference
	resInstance := builder.Multi
	reconcilers, err := resInstance.Build(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, reconciler := range reconcilers {
		resource, err := reconciler.Build(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		// resInstance reconcile
		// return b.DoReconcile(ctx, resource)
		if handler, ok := reconciler.(ResourceHandler); ok {
			_, err := handler.DoReconcile(ctx, resource, handler)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			panic(fmt.Sprintf("resource is not ResourceHandler, actual is - %T", resource))
		}
	}

	if validator, ok := resInstance.(ResourceValidator); ok {
		return validator.Validate(ctx)
	}
	return ctrl.Result{}, nil
}
