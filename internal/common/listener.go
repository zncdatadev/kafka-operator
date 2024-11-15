package common

import (
	"fmt"
	"strconv"
	"strings"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

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

func GetKafkaListenerConfig(namespace string, kafkaSecurity *security.KafkaTlsSecurity, objectName string) (*KafkaListenerConfig, error) {
	podFqdn := podFqdn(namespace, objectName)

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
			Host: LISTENER_NODE_ADDRESS,
			Port: nodePortCmd(kafkav1alpha1.KubedoopTmpDir),
		})
		listenerSecurityProtocolMap[ClientAuth] = Ssl
	} else if kafkaSecurity.TlsServerSecretClass() != "" {
		listeners = append(listeners, KafkaListener{
			Name: Client,
			Host: LISTENER_LOCAL_ADDRESS,
			Port: strconv.Itoa(kafkaSecurity.ClientPort()),
		})
		advertisedListeners = append(advertisedListeners, KafkaListener{
			Name: Client,
			Host: LISTENER_NODE_ADDRESS,
			Port: nodePortCmd(kafkav1alpha1.KubedoopTmpDir),
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
			Host: LISTENER_NODE_ADDRESS,
			Port: nodePortCmd(kafkav1alpha1.KubedoopTmpDir),
		})
		listenerSecurityProtocolMap[Client] = Plaintext
	}

	if kafkaSecurity.TlsInternalSecretClass() != "" {
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

	return &KafkaListenerConfig{
		Listeners:                   listeners,
		AdvertisedListeners:         advertisedListeners,
		ListenerSecurityProtocolMap: listenerSecurityProtocolMap,
	}, nil
}

func nodePortCmd(directory string) string {
	return fmt.Sprintf("$(cat %s/%s)", directory, NodePortFileName)
}

func podFqdn(namespace string, objectName string) string {
	return fmt.Sprintf("$POD_NAME.%s.%s.svc.cluster.local", objectName, namespace)
}
