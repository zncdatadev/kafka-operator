package config

import (
	"golang.org/x/exp/maps"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
)

// server.properties

// KafkaServerConfGenerator kafka config generator
type KafkaServerConfGenerator struct {
	KafkaTlsSecurity *security.KafkaTlsSecurity
}

// example:
// controlled.shutdown.enable=true
// inter.broker.listener.name=INTERNAL
// listener.name.internal.ssl.client.auth=required
// listener.name.internal.ssl.keystore.location=/stackroot/tls_keystore_internal/keystore.p12
// listener.name.internal.ssl.keystore.password=
// listener.name.internal.ssl.keystore.type=PKCS12
// listener.name.internal.ssl.truststore.location=/stackroot/tls_keystore_internal/truststore.p12
// listener.name.internal.ssl.truststore.password=
// listener.name.internal.ssl.truststore.type=PKCS12
// log.dirs=/stackroot/data/topicdata
// zookeeper.connect=localhost\:2181
// zookeeper.connection.timeout.ms=18000

func (k *KafkaServerConfGenerator) Generate() (string, error) {
	serverConfig := map[string]string{
		"zookeeper.connection.timeout.ms": "18000",
	}
	maps.Copy(serverConfig, k.gracefulShutdownConfigProperties()) // shutdown
	maps.Copy(serverConfig, k.KafkaTlsSecurity.ConfigSettings())  // tls

	serverConfig["log.dirs"] = k.DataDir()

	content := util.ToProperties(serverConfig)
	return content, nil
}

func (k *KafkaServerConfGenerator) FileName() string {
	return kafkav1alpha1.ServerFileName
}

// We don't specify other configs (such as controlled.shutdown.retry.backoff.ms and controlled.shutdown.max.retries),
// as this way we can benefit from changing defaults in the future.
func (k *KafkaServerConfGenerator) gracefulShutdownConfigProperties() map[string]string {
	return map[string]string{
		"controlled.shutdown.enable": "true",
	}
}

// DataDir data dir
func (k *KafkaServerConfGenerator) DataDir() string {
	return kafkav1alpha1.KubedoopDataDir + "/topicdata"
}
