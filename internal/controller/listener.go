package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zncdatadev/kafka-operator/internal/pkg"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

func NewRoleGroupBootstrapListenerReconciler(
	client *client.Client,
	bootstrapListenerClass string,
	info *reconciler.RoleGroupInfo,
	kafkaTlsSecurity *security.KafkaSecurity,
) reconciler.ResourceReconciler[pkg.ListenerBuidler] {

	builder := pkg.NewListenerBuilder(
		client,
		BootstrapListenerName(info),
		bootstrapListenerClass,

		func(lbo *pkg.ListenerBuilderOptions) {
			lbo.ContainerPorts = KafkaContainerPorts(kafkaTlsSecurity)
			lbo.PublishNotReadyAddresses = true
			lbo.ExtraPodSelectorLabels = map[string]string{
				LabelListenerBootstrap: LabelListenerBootstrapValue, // "app.kubernetes.io/listener-bootstrap: true", add this label for search in discovery
			}
		},
	)

	return reconciler.NewGenericResourceReconciler(client, builder)
}

const (
	LISTENER_LOCAL_ADDRESS = "0.0.0.0"
	LISTENER_NODE_ADDRESS  = "$NODE"
)

type KafkaListenerError struct {
	Message string
}

func (e *KafkaListenerError) Error() string {
	return e.Message
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
	for _, listener := range config.Listeners {
		listeners = append(listeners, listener.String())
	}
	return strings.Join(listeners, ",")
}

func (config *KafkaListenerConfig) AdvertisedListenersString() string {
	advertisedListeners := make([]string, 0, len(config.AdvertisedListeners))
	for _, listener := range config.AdvertisedListeners {
		advertisedListeners = append(advertisedListeners, listener.String())
	}
	return strings.Join(advertisedListeners, ",")
}

func (config *KafkaListenerConfig) ListenerSecurityProtocolMapString() string {
	protocolMap := make([]string, 0, len(config.ListenerSecurityProtocolMap))
	for name, protocol := range config.ListenerSecurityProtocolMap {
		protocolMap = append(protocolMap, fmt.Sprintf("%s:%s", name, protocol))
	}
	return strings.Join(protocolMap, ",")
}

func GetKafkaListenerConfig(
	namespace string,
	kafkaSecurity *security.KafkaSecurity,
	objectName string,
) (*KafkaListenerConfig, error) {
	podFqdn := util.PodFqdn(namespace, objectName)

	var listeners []KafkaListener
	var advertisedListeners []KafkaListener
	listenerSecurityProtocolMap := make(map[KafkaListenerName]KafkaListenerProtocol)

	if kafkaSecurity.TlsClientAuthenticationClass() != "" {
		listeners = append(listeners, KafkaListener{
			Name: ClientAuth,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.ClientPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: ClientAuth,
			Host: util.NodeAddressCmd(kafkav1alpha1.KubedoopListenerBrokerDir),
			Port: util.NodePortCmd(kafkav1alpha1.KubedoopListenerBrokerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[ClientAuth] = Ssl
	} else if kafkaSecurity.IsKerberosEnabled() {
		// 1) Kerberos authentication is enabled
		listeners = append(listeners, KafkaListener{
			Name: Client,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.ClientPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Client,
			Host: util.NodeAddressCmd(kafkav1alpha1.KubedoopListenerBrokerDir),
			Port: util.NodePortCmd(kafkav1alpha1.KubedoopListenerBrokerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[Client] = Ssl
	} else if kafkaSecurity.TlsServerSecretClass() != "" {
		listeners = append(listeners, KafkaListener{
			Name: Client,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.ClientPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Client,
			Host: util.NodeAddressCmd(kafkav1alpha1.KubedoopListenerBrokerDir),
			Port: util.NodePortCmd(kafkav1alpha1.KubedoopListenerBrokerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[Client] = Ssl
	} else {
		listeners = append(listeners, KafkaListener{
			Name: Client,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkav1alpha1.ClientPort),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Client,
			Host: util.NodeAddressCmd(kafkav1alpha1.KubedoopListenerBrokerDir),
			Port: util.NodePortCmd(kafkav1alpha1.KubedoopListenerBrokerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[Client] = Plaintext
	}

	if kafkaSecurity.TlsInternalSecretClass() != "" || kafkaSecurity.IsKerberosEnabled() {
		listeners = append(listeners, KafkaListener{
			Name: Internal,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.InternalPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Internal,
			Host: podFqdn,
			Port: strconv.Itoa(kafkaSecurity.InternalPort()),
		})
		listenerSecurityProtocolMap[Internal] = Ssl
	} else {
		listeners = append(listeners, KafkaListener{
			Name: Internal,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.InternalPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Internal,
			Host: podFqdn,
			Port: strconv.Itoa(kafkaSecurity.InternalPort()),
		})
		listenerSecurityProtocolMap[Internal] = Plaintext
	}

	// BOOTSTRAP
	// if kafka_security.has_kerberos_enabled() {
	//     listeners.push(KafkaListener {
	//         name: KafkaListenerName::Bootstrap,
	//         host: LISTENER_LOCAL_ADDRESS.to_string(),
	//         port: kafka_security.bootstrap_port().to_string(),
	//     });
	//     advertised_listeners.push(KafkaListener {
	//         name: KafkaListenerName::Bootstrap,
	//         host: node_address_cmd(STACKABLE_LISTENER_BROKER_DIR),
	//         port: node_port_cmd(
	//             STACKABLE_LISTENER_BROKER_DIR,
	//             kafka_security.client_port_name(),
	//         ),
	//     });
	//     listener_security_protocol_map
	//         .insert(KafkaListenerName::Bootstrap, KafkaListenerProtocol::SaslSsl);
	// }

	// Bootstrap
	if kafkaSecurity.IsKerberosEnabled() {
		listeners = append(listeners, KafkaListener{
			Name: Bootstrap,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.BootstrapPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Bootstrap,
			Host: util.NodeAddressCmd(kafkav1alpha1.KubedoopListenerBrokerDir),
			Port: util.NodePortCmd(kafkav1alpha1.KubedoopListenerBrokerDir, kafkaSecurity.ClientPortName()),
		})
		listenerSecurityProtocolMap[Bootstrap] = Ssl
	}

	return &KafkaListenerConfig{
		Listeners:                   listeners,
		AdvertisedListeners:         advertisedListeners,
		ListenerSecurityProtocolMap: listenerSecurityProtocolMap,
	}, nil
}
