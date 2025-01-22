package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/zncdatadev/kafka-operator/internal/security"
	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KafkaDiscoveryKey = "KAFKA"

	LabelListenerBootstrap      = "app.kubernetes.io/listener-bootstrap"
	LabelListenerBootstrapValue = "true"
)

type DiscoveryBuilder struct {
	builder.ConfigMapBuilder
	kafkaSecurity *security.KafkaTlsSecurity
	isNodePort    bool
}

func NewKafkaDiscoveryReconciler(
	ctx context.Context,
	client *client.Client,
	kafkaTlsSecurity *security.KafkaTlsSecurity,
) reconciler.ResourceReconciler[builder.ConfigBuilder] {
	builder := NewKafkaDiscoveryBuilder(client, kafkaTlsSecurity, false)
	return reconciler.NewGenericResourceReconciler(client, builder)
}

func NewKafkaDiscoveryNodePortReconciler(
	ctx context.Context,
	client *client.Client,
	kafkaTlsSecurity *security.KafkaTlsSecurity,
) reconciler.ResourceReconciler[builder.ConfigBuilder] {
	builder := NewKafkaDiscoveryBuilder(client, kafkaTlsSecurity, true)
	return reconciler.NewGenericResourceReconciler(client, builder)
}

func NewKafkaDiscoveryBuilder(
	client *client.Client,
	kafkaSecurity *security.KafkaTlsSecurity,
	isNodePort bool,
) builder.ConfigBuilder {
	name := client.GetOwnerName()
	if isNodePort {
		name = fmt.Sprintf("%s-nodeport", name)
	}

	return &DiscoveryBuilder{
		ConfigMapBuilder: *builder.NewConfigMapBuilder(
			client,
			name,
			func(o *builder.Options) {
				o.Labels = client.OwnerReference.GetLabels()
			},
		),
		kafkaSecurity: kafkaSecurity,
		isNodePort:    isNodePort,
	}
}

func (b *DiscoveryBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	portName := b.kafkaSecurity.ClientPortName()

	listenerList := &listenerv1alpha1.ListenerList{}
	err := b.Client.Client.List(
		ctx,
		listenerList,
		ctrlclient.MatchingLabels{LabelListenerBootstrap: LabelListenerBootstrapValue},
	)
	if err != nil {
		return nil, err
	}

	hosts, err := b.listenerHosts(listenerList, portName)
	if err != nil {
		return nil, err
	}

	bootstrapServers := b.makeBootstrapServers(hosts)
	b.AddItem(KafkaDiscoveryKey, bootstrapServers)

	return b.GetObject(), nil
}

type HostPort struct {
	Host string
	Port int32
}

func (b *DiscoveryBuilder) listenerHosts(listenerList *listenerv1alpha1.ListenerList, portName string) ([]HostPort, error) {
	var result []HostPort
	for _, listener := range listenerList.Items {
		// TODO: Status refactor to user pointer
		if listener.Status.IngressAddresses == nil {
			continue
		}
		for _, addr := range listener.Status.IngressAddresses {
			port, ok := addr.Ports[portName]
			if !ok {
				return nil, &Error{msg: fmt.Sprintf("no service port with name %s", portName)}
			}
			result = append(result, HostPort{
				Host: addr.Address,
				Port: port,
			})
		}
	}
	return result, nil
}

func (b *DiscoveryBuilder) makeBootstrapServers(hosts []HostPort) string {
	var servers = make([]string, 0, len(hosts))
	for _, h := range hosts {
		servers = append(servers, fmt.Sprintf("%s:%d", h.Host, h.Port))
	}
	return strings.Join(servers, ",")
}

type Error struct {
	msg string
	err error
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.err)
	}
	return e.msg
}
