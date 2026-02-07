package pkg

import (
	"context"
	"maps"

	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ListenerBuidler interface {
	builder.ObjectBuilder
}

type ListenerBuilderOptions struct {
	builder.Options

	ContainerPorts           []corev1.ContainerPort
	PublishNotReadyAddresses bool
	ExtraPodSelectorLabels   map[string]string
}

type ServiceBuilderOption func(*ListenerBuilderOptions)

func NewListenerBuilder(
	client *client.Client,
	name string,
	listenerClass string,
	options ...ServiceBuilderOption,
) ListenerBuidler {

	opts := &ListenerBuilderOptions{}
	for _, opt := range options {
		opt(opts)
	}

	return &GenericBuilder{
		ObjectMeta:             builder.NewObjectMeta(client, name),
		listenerClass:          listenerClass,
		ports:                  containerPortToListenerPort(opts.ContainerPorts),
		extraPodSelectorLabels: opts.ExtraPodSelectorLabels,
		publishNotReadyAddress: opts.PublishNotReadyAddresses,
	}

}

func containerPortToListenerPort(port []corev1.ContainerPort) []listenerv1alpha1.PortSpec {
	ports := make([]listenerv1alpha1.PortSpec, 0, len(port))
	for _, p := range port {
		ports = append(ports, listenerv1alpha1.PortSpec{
			Name:     p.Name,
			Port:     p.ContainerPort,
			Protocol: p.Protocol,
		})
	}

	return ports
}

var _ ListenerBuidler = &GenericBuilder{}

type GenericBuilder struct {
	*builder.ObjectMeta

	listenerClass          string
	ports                  []listenerv1alpha1.PortSpec
	extraPodSelectorLabels map[string]string
	publishNotReadyAddress bool
}

// GetObject implements ListenerBuidler.
func (g *GenericBuilder) GetObject() ctrlclient.Object {
	objectMeta := g.GetObjectMeta()

	lables := objectMeta.GetLabels()
	listenerLabels := maps.Clone(lables)
	if g.extraPodSelectorLabels != nil {
		for k, v := range g.extraPodSelectorLabels {
			listenerLabels[k] = v
		}
	}
	objectMeta.SetLabels(listenerLabels)

	return &listenerv1alpha1.Listener{
		ObjectMeta: objectMeta,
		Spec: listenerv1alpha1.ListenerSpec{
			ClassName: g.listenerClass,
			Ports:     g.ports,
		},
	}
}

// Build implements ListenerBuidler.
func (g *GenericBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	return g.GetObject(), nil
}

// set listener class
func (g *GenericBuilder) SetListenerClass(listenerClass string) {
	g.listenerClass = listenerClass
}

// set ports
func (g *GenericBuilder) SetPorts(ports []listenerv1alpha1.PortSpec) {
	g.ports = ports
}

// set extra pod selector labels
func (g *GenericBuilder) SetExtraPodSelectorLabels(labels map[string]string) {
	g.extraPodSelectorLabels = labels
}

// set publish not ready address
func (g *GenericBuilder) SetPublishNotReadyAddresses(publishNotReadyAddresses bool) {
	g.publishNotReadyAddress = publishNotReadyAddresses
}

// add port
func (g *GenericBuilder) AddPort(port listenerv1alpha1.PortSpec) {
	g.ports = append(g.ports, port)
}

// add extra pod selector label
func (g *GenericBuilder) AddExtraPodSelectorLabel(key, value string) {
	if g.extraPodSelectorLabels == nil {
		g.extraPodSelectorLabels = make(map[string]string)
	}
	g.extraPodSelectorLabels[key] = value
}

// get listener class
func (g *GenericBuilder) GetListenerClass() string {
	return g.listenerClass
}

// get ports
func (g *GenericBuilder) GetPorts() []listenerv1alpha1.PortSpec {
	return g.ports
}

// get extra pod selector labels
func (g *GenericBuilder) GetExtraPodSelectorLabels() map[string]string {
	return g.extraPodSelectorLabels
}

// get publish not ready address
func (g *GenericBuilder) GetPublishNotReadyAddresses() bool {
	return g.publishNotReadyAddress
}
