package controller

import (
	"maps"

	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/constants"
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
	svcLabels["prometheus.io/scrape"] = "true"

	builder := NewServiceBuilder(
		client,
		info.GetFullName(),
		nil,
		func(sbo *builder.ServiceBuilderOptions) {
			sbo.Headless = true
			sbo.ListenerClass = constants.ClusterInternal
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
