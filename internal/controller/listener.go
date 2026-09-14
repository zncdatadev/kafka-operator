package controller

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/listener"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

const (
	ListenerLocalAddress = "0.0.0.0"
)

// NodeAddressCmd returns the shell substitution reading the listener address published by
// listener-operator into the given listener volume directory.
func NodeAddressCmd(directory string) string {
	return fmt.Sprintf("$(cat %s)", path.Join(directory, "default-address/address"))
}

// NodePortCmd returns the shell substitution reading the published port with the given
// name from the listener volume directory.
func NodePortCmd(directory string, portName string) string {
	return fmt.Sprintf("$(cat %s)", path.Join(directory, "default-address/ports", portName))
}

// PodFqdn returns the pod FQDN under the role group's headless service (evaluated by the
// shell at container start via $POD_NAME).
func PodFqdn(buildCtx *reconciler.RoleGroupBuildContext) string {
	return fmt.Sprintf("$POD_NAME.%s-headless.%s.svc.cluster.local",
		buildCtx.ResourceName, buildCtx.ClusterNamespace)
}

// BootstrapListenerName is the name of the per-role-group bootstrap Listener CR.
func BootstrapListenerName(resourceName string) string {
	return resourceName + "-bootstrap"
}

// KafkaContainerPorts returns the main container ports: client, metrics, and the
// bootstrap port when Kerberos is enabled.
func KafkaContainerPorts(kafkaSecurity *security.KafkaSecurity) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          kafkaSecurity.ClientPortName(),
			ContainerPort: int32(kafkaSecurity.ClientPort()),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          kafkav1alpha1.MetricsPortName,
			ContainerPort: kafkav1alpha1.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	if kafkaSecurity.IsKerberosEnabled() {
		ports = append(ports, corev1.ContainerPort{
			Name:          kafkav1alpha1.BootstrapPortName,
			ContainerPort: int32(kafkaSecurity.BootstrapPort()),
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return ports
}

// kafkaServicePorts returns the ports exposed by the role group services.
func kafkaServicePorts(kafkaSecurity *security.KafkaSecurity) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, 3)
	for _, p := range KafkaContainerPorts(kafkaSecurity) {
		ports = append(ports, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.ContainerPort,
			TargetPort: intstr.FromInt32(p.ContainerPort),
			Protocol:   p.Protocol,
		})
	}
	return ports
}

// NewBootstrapListener builds the per-role-group bootstrap Listener CR. It is labeled
// with LabelListenerBootstrap so discovery can aggregate all bootstrap addresses of the
// cluster, and carries the same identity labels as the other role group resources.
func NewBootstrapListener(
	name string,
	namespace string,
	labels map[string]string,
	listenerClass string,
	kafkaSecurity *security.KafkaSecurity,
) ctrlclient.Object {
	listenerLabels := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		listenerLabels[k] = v
	}
	listenerLabels[LabelListenerBootstrap] = LabelValueTrue

	ports := make([]listenerv1alpha1.PortSpec, 0, 3)
	for _, p := range KafkaContainerPorts(kafkaSecurity) {
		ports = append(ports, listenerv1alpha1.PortSpec{
			Name:     p.Name,
			Port:     p.ContainerPort,
			Protocol: p.Protocol,
		})
	}

	return &listenerv1alpha1.Listener{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    listenerLabels,
		},
		Spec: listenerv1alpha1.ListenerSpec{
			ClassName:                listenerClass,
			Ports:                    ports,
			PublishNotReadyAddresses: ptr.To(true),
		},
	}
}

type KafkaListenerProtocol string

const (
	Plaintext KafkaListenerProtocol = "PLAINTEXT"
	Ssl       KafkaListenerProtocol = "SSL"
)

type KafkaListenerName string

const (
	Client     KafkaListenerName = "CLIENT"
	ClientAuth KafkaListenerName = "CLIENT_AUTH"
	Internal   KafkaListenerName = "INTERNAL"
	Bootstrap  KafkaListenerName = "BOOTSTRAP"
)

type KafkaListener struct {
	Name KafkaListenerName
	Host string
	Port string
}

func (kl KafkaListener) String() string {
	return fmt.Sprintf("%s://%s:%s", kl.Name, kl.Host, kl.Port)
}

type KafkaListenerConfig struct {
	Listeners                   []KafkaListener
	AdvertisedListeners         []KafkaListener
	ListenerSecurityProtocolMap map[KafkaListenerName]KafkaListenerProtocol
}

func (config *KafkaListenerConfig) ListenersString() string {
	listeners := make([]string, 0, len(config.Listeners))
	for _, l := range config.Listeners {
		listeners = append(listeners, l.String())
	}
	return strings.Join(listeners, ",")
}

func (config *KafkaListenerConfig) AdvertisedListenersString() string {
	advertisedListeners := make([]string, 0, len(config.AdvertisedListeners))
	for _, l := range config.AdvertisedListeners {
		advertisedListeners = append(advertisedListeners, l.String())
	}
	return strings.Join(advertisedListeners, ",")
}

func (config *KafkaListenerConfig) ListenerSecurityProtocolMapString() string {
	names := make([]KafkaListenerName, 0, len(config.ListenerSecurityProtocolMap))
	for name := range config.ListenerSecurityProtocolMap {
		names = append(names, name)
	}
	// Stable output: CLIENT/CLIENT_AUTH first, then INTERNAL, then BOOTSTRAP.
	order := map[KafkaListenerName]int{Client: 0, ClientAuth: 0, Internal: 1, Bootstrap: 2}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if order[names[j]] < order[names[i]] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	protocolMap := make([]string, 0, len(names))
	for _, name := range names {
		protocolMap = append(protocolMap, fmt.Sprintf("%s:%s", name, config.ListenerSecurityProtocolMap[name]))
	}
	return strings.Join(protocolMap, ",")
}

// GetKafkaListenerConfig computes the Kafka listeners, advertised listeners and the
// security protocol map from the security configuration. Advertised client/bootstrap
// addresses are read at container start from the listener volumes (shell substitution);
// the internal listener advertises the pod FQDN under the headless service.
func GetKafkaListenerConfig(
	buildCtx *reconciler.RoleGroupBuildContext,
	kafkaSecurity *security.KafkaSecurity,
	listenerProvisioner *listener.ListenerProvisioner,
) (*KafkaListenerConfig, error) {
	podFqdn := PodFqdn(buildCtx)
	brokerListenerDir := listenerProvisioner.MustPath(kafkav1alpha1.ListenerBrokerVolumeName)

	var listeners []KafkaListener
	var advertisedListeners []KafkaListener
	listenerSecurityProtocolMap := make(map[KafkaListenerName]KafkaListenerProtocol)

	clientPort := strconv.Itoa(kafkaSecurity.ClientPort())
	advertisedClient := KafkaListener{
		Host: NodeAddressCmd(brokerListenerDir),
		Port: NodePortCmd(brokerListenerDir, kafkaSecurity.ClientPortName()),
	}

	switch {
	case kafkaSecurity.TlsClientAuthenticationClass() != "":
		listeners = append(listeners, KafkaListener{Name: ClientAuth, Host: ListenerLocalAddress, Port: clientPort})
		advertisedClient.Name = ClientAuth
		advertisedListeners = append(advertisedListeners, advertisedClient)
		listenerSecurityProtocolMap[ClientAuth] = Ssl
	case kafkaSecurity.IsKerberosEnabled():
		listeners = append(listeners, KafkaListener{Name: Client, Host: ListenerLocalAddress, Port: clientPort})
		advertisedClient.Name = Client
		advertisedListeners = append(advertisedListeners, advertisedClient)
		listenerSecurityProtocolMap[Client] = Ssl
	case kafkaSecurity.TlsServerSecretClass() != "":
		listeners = append(listeners, KafkaListener{Name: Client, Host: ListenerLocalAddress, Port: clientPort})
		advertisedClient.Name = Client
		advertisedListeners = append(advertisedListeners, advertisedClient)
		listenerSecurityProtocolMap[Client] = Ssl
	default:
		listeners = append(listeners, KafkaListener{Name: Client, Host: ListenerLocalAddress, Port: clientPort})
		advertisedClient.Name = Client
		advertisedListeners = append(advertisedListeners, advertisedClient)
		listenerSecurityProtocolMap[Client] = Plaintext
	}

	// INTERNAL (broker-to-broker) listener.
	internalPort := strconv.Itoa(kafkaSecurity.InternalPort())
	listeners = append(listeners, KafkaListener{Name: Internal, Host: ListenerLocalAddress, Port: internalPort})
	advertisedListeners = append(advertisedListeners, KafkaListener{Name: Internal, Host: podFqdn, Port: internalPort})
	if kafkaSecurity.TlsInternalSecretClass() != "" || kafkaSecurity.IsKerberosEnabled() {
		listenerSecurityProtocolMap[Internal] = Ssl
	} else {
		listenerSecurityProtocolMap[Internal] = Plaintext
	}

	// BOOTSTRAP listener (Kerberos only).
	if kafkaSecurity.IsKerberosEnabled() {
		listeners = append(listeners, KafkaListener{
			Name: Bootstrap,
			Host: ListenerLocalAddress,
			Port: strconv.Itoa(kafkaSecurity.BootstrapPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Bootstrap,
			Host: NodeAddressCmd(brokerListenerDir),
			Port: NodePortCmd(brokerListenerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[Bootstrap] = Ssl
	}

	return &KafkaListenerConfig{
		Listeners:                   listeners,
		AdvertisedListeners:         advertisedListeners,
		ListenerSecurityProtocolMap: listenerSecurityProtocolMap,
	}, nil
}
