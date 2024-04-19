package common

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SinglePodServiceResourceType interface {
	MultiResourceReconcilerBuilder
}

// ServiceBuilder service builder
// contains: name, namespace, labels, ports, these should be required
// optional: clusterIP, serviceType should be optional,
type ServiceBuilder struct {
	Name      string
	Namespace string
	Labels    map[string]string
	selector  map[string]string
	Ports     []corev1.ServicePort

	ClusterIP *HeadlessServiceType
	Type      *corev1.ServiceType
}

func NewServiceBuilder(
	name string,
	namespace string,
	labels map[string]string,
	ports []corev1.ServicePort,
) *ServiceBuilder {
	return &ServiceBuilder{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		Ports:     ports,
	}
}

func (s *ServiceBuilder) SetClusterIP(ip *HeadlessServiceType) *ServiceBuilder {
	s.ClusterIP = ip
	return s
}

func (s *ServiceBuilder) SetType(t *corev1.ServiceType) *ServiceBuilder {
	s.Type = t
	return s
}

// SetSelector set select labels
func (s *ServiceBuilder) SetSelector(labels map[string]string) *ServiceBuilder {
	s.selector = labels
	return s
}

func (s *ServiceBuilder) Build() *corev1.Service {
	if s.selector == nil {
		s.selector = s.Labels
	}
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    s.Labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    s.Ports,
			Selector: s.selector,
		},
	}

	if s.ClusterIP != nil {
		svc.Spec.ClusterIP = string(*s.ClusterIP)
	}
	if s.Type != nil {
		svc.Spec.Type = *s.Type
	}
	return &svc
}

type HeadlessServiceType string

const (
	Service         HeadlessServiceType = ""
	HeadlessService HeadlessServiceType = "None"
)

type GenericServiceReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	svcBuilder *ServiceBuilder
}

func NewGenericServiceReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	svcBuilder *ServiceBuilder,
) *GenericServiceReconciler[T, G] {
	return &GenericServiceReconciler[T, G]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
		svcBuilder: svcBuilder,
	}
}

func (s *GenericServiceReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	return s.svcBuilder.Build(), nil
}
