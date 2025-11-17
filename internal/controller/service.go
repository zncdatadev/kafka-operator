package controller

import (
	"maps"
	"strconv"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	opconstants "github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
)

// RoleGroup Service is a headless service that enables:
// 1. Direct access to specific rolegroup instances
// 2. Internal peer-to-peer communication
// 3. Client-side load balancing capabilities

func NewRoleGroupService(
	client *client.Client,
	info *reconciler.RoleGroupInfo,
) reconciler.ResourceReconciler[builder.ServiceBuilder] {

	matchLabels := info.GetLabels()
	svcLabels := maps.Clone(matchLabels)
	svcLabels["prometheus.io/scrape"] = LabelValueTrue

	builder := NewServiceBuilder(
		client,
		info.GetFullName(),
		nil,
		func(sbo *builder.ServiceBuilderOptions) {
			sbo.Headless = true
			sbo.ListenerClass = opconstants.ClusterInternal
			sbo.Labels = svcLabels
			sbo.MatchingLabels = matchLabels
		},
	)

	return reconciler.NewGenericResourceReconciler(
		client,
		builder,
	)
}

func NewServiceBuilder(
	client *client.Client,
	name string,
	ports []corev1.ContainerPort,
	options ...builder.ServiceBuilderOption,
) builder.ServiceBuilder {
	return &ServiceBuilder{
		BaseServiceBuilder: builder.NewServiceBuilder(
			client,
			name,
			ports,
			options...,
		),
	}
}

var _ builder.ServiceBuilder = &ServiceBuilder{}

type ServiceBuilder struct {
	*builder.BaseServiceBuilder
}

func (b *ServiceBuilder) GetObject() *corev1.Service {
	obj := b.BaseServiceBuilder.GetObject()
	obj.Spec.PublishNotReadyAddresses = true // PublishNotReadyAddresses indicates that any agent can access the service even if it is not ready.
	return obj
}

// NewRoleGroupMetricsService creates a metrics service reconciler using a simple function approach
// This creates a headless service for metrics with Prometheus labels and annotations
func NewRoleGroupMetricsService(
	client *client.Client,
	roleGroupInfo *reconciler.RoleGroupInfo,
) reconciler.Reconciler {
	// Get metrics port
	metricsPort := kafkav1alpha1.MetricsPort

	// Create service ports
	servicePorts := []corev1.ContainerPort{
		{
			Name:          kafkav1alpha1.MetricsPortName,
			ContainerPort: int32(metricsPort),
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Create service name with -metrics suffix
	serviceName := CreateServiceMetricsName(roleGroupInfo)

	scheme := "http"

	// Prepare labels (copy from roleGroupInfo and add metrics labels)
	labels := make(map[string]string)
	for k, v := range roleGroupInfo.GetLabels() {
		labels[k] = v
	}
	labels["prometheus.io/scrape"] = LabelValueTrue

	// Prepare annotations (copy from roleGroupInfo and add Prometheus annotations)
	annotations := make(map[string]string)
	for k, v := range roleGroupInfo.GetAnnotations() {
		annotations[k] = v
	}
	annotations["prometheus.io/scrape"] = LabelValueTrue
	// annotations["prometheus.io/path"] = "/metrics"  // default path is /metrics
	annotations["prometheus.io/port"] = strconv.Itoa(metricsPort)
	annotations["prometheus.io/scheme"] = scheme

	// Create base service builder
	baseBuilder := builder.NewServiceBuilder(
		client,
		serviceName,
		servicePorts,
		func(sbo *builder.ServiceBuilderOptions) {
			sbo.Headless = true
			sbo.ListenerClass = opconstants.ClusterInternal
			sbo.Labels = labels
			sbo.MatchingLabels = roleGroupInfo.GetLabels() // Use original labels for matching
			sbo.Annotations = annotations
		},
	)

	return reconciler.NewGenericResourceReconciler(
		client,
		baseBuilder,
	)
}

func CreateServiceMetricsName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName() + "-metrics"
}
