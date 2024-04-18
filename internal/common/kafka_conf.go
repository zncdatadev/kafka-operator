package common

import (
	"fmt"
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"strings"
)

// log4j.properties

// Log4jConfGenerator kafka log4j conf generator
type Log4jConfGenerator struct {
}

func (g *Log4jConfGenerator) Generate() string {
	return `log4j.rootLogger=INFO, CONSOLE, FILE

log4j.appender.CONSOLE=org.apache.log4j.ConsoleAppender
log4j.appender.CONSOLE.Threshold=INFO
log4j.appender.CONSOLE.layout=org.apache.log4j.PatternLayout
log4j.appender.CONSOLE.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.FILE=org.apache.log4j.RollingFileAppender
log4j.appender.FILE.Threshold=INFO
log4j.appender.FILE.File=/bitnami/log/kafka.log
log4j.appender.FILE.MaxFileSize=5MB
log4j.appender.FILE.MaxBackupIndex=1
log4j.appender.FILE.layout=org.apache.log4j.PatternLayout
`
}

func (g *Log4jConfGenerator) FileName() string {
	return v1alpha1.Log4jFileName
}

// security.properties

// SecurityConfGenerator kafka security conf generator
type SecurityConfGenerator struct {
}

func (g *SecurityConfGenerator) Generate() string {
	return `networkaddress.cache.negative.ttl=0
networkaddress.cache.ttl=30`
}

func (g *SecurityConfGenerator) FileName() string {
	return v1alpha1.SecurityFileName
}

//server.properties

// KafkaConfGenerator kafka config generator
type KafkaConfGenerator struct {
	SslSpec *v1alpha1.SslSpec
}

const serverProperties = `
controlled.shutdown.enable=true
zookeeper.connection.timeout.ms=18000
`

//controlled.shutdown.enable=true
//inter.broker.listener.name=INTERNAL
//listener.name.internal.ssl.client.auth=required
//listener.name.internal.ssl.keystore.location=/stackable/tls_keystore_internal/keystore.p12
//listener.name.internal.ssl.keystore.password=
//listener.name.internal.ssl.keystore.type=PKCS12
//listener.name.internal.ssl.truststore.location=/stackable/tls_keystore_internal/truststore.p12
//listener.name.internal.ssl.truststore.password=
//listener.name.internal.ssl.truststore.type=PKCS12
//log.dirs=/stackable/data/topicdata
//zookeeper.connect=localhost\:2181
//zookeeper.connection.timeout.ms=18000

func (k *KafkaConfGenerator) Generate() string {
	return serverProperties + "\n" +
		"listeners=" + k.Listeners() + "\n" +
		"inter.broker.listener.name=" + k.InterBrokerListenerName() + "\n" +
		k.listenerSslProperties(k.SslSpec) + "\n" +
		"log.dirs=" + k.DataDir() + "\n" +
		//k.zkConnections() + "\n" +
		"listener.security.protocol.map=" + k.ListenerSecurityProtocolMap(k.SslSpec)
}

func (k *KafkaConfGenerator) FileName() string {
	return v1alpha1.ServerFileName
}

const interListerName = INTERNAL

// InterBrokerListenerName inter broker listener name
func (k *KafkaConfGenerator) InterBrokerListenerName() string {
	return string(interListerName)
}

// listener prefix
func (k *KafkaConfGenerator) listenerPrefix(listenerName ListenerName) string {
	return fmt.Sprintf("listener.name.%s.", strings.ToLower(string(listenerName)))
}

// Listeners listeners
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listeners=CLIENT://:9092,ClUSTER://:19092,EXTERNAL://:{nodePort}
func (k *KafkaConfGenerator) Listeners() string {
	return fmt.Sprintf("%s://:%d,%s://:%d", CLIENT, v1alpha1.KafkaClientPort,
		INTERNAL, v1alpha1.InternalPort)
}

// ListenerSecurityProtocolMap
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listener.security.protocol.map=CLIENT:PLAINTEXT,CLUSTER:PLAINTEXT,EXTERNAL:PLAINTEXT
func (k *KafkaConfGenerator) ListenerSecurityProtocolMap(sslSpec *v1alpha1.SslSpec) string {
	var protocol SecurityProtocol
	if SslEnabled(sslSpec) {
		protocol = Ssl
	} else {
		protocol = Plaintext
	}
	return fmt.Sprintf("%s:PLAINTEXT,%s:%s", CLIENT, INTERNAL, string(protocol))
}

// DataDir data dir
// ex: log.dirs=/var/lib/zookeeper/data
func (k *KafkaConfGenerator) DataDir() string {
	return DataMountPath + "/data/topicdata"
}

// zk connections

// zk connections
// ex: zookeeper.connect=zookeeper-service:2181
func (k *KafkaConfGenerator) zkConnections() string {
	return fmt.Sprintf("zookeeper.connect=localhost:2180")
}

// listener ssl properties
// ex:
// ssl.keystore.location=/opt/kafka/config/keystore.jks
// ssl.keystore.password=${env.KAFKA_SSL_KEYSTORE_PASSWORD}
// ssl.key.password=${env.KAFKA_SSL_KEY_PASSWORD}
// ssl.truststore.location=/opt/kafka/config/truststore.jks
// ssl.truststore.password=${env.KAFKA_SSL_TRUSTSTORE_PASSWORD}
func (k *KafkaConfGenerator) listenerSslProperties(sslSpec *v1alpha1.SslSpec) string {
	if SslEnabled(sslSpec) {
		return fmt.Sprintf(k.listenerPrefix(interListerName) + ".ssl.keystore.location=" + TlsKeystoreMountPath + "/keystore.jks\n" +
			k.listenerPrefix(interListerName) + "ssl.keystore.password=" + sslSpec.KeyStorePassword + "\n" +
			k.listenerPrefix(interListerName) + "ssk.keystore.type=" + sslSpec.KeyStoreType + "\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.location=" + TlsKeystoreMountPath + "/truststore.jks\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.password=" + sslSpec.KeyStorePassword + "\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.type=" + sslSpec.KeyStoreType)
	}
	return ""
}

func TlsKeystoreInternalVolumeName() string {
	return "tls-keystore-internal"
}

const TlsDir = "tls-keystore-internal"

const (
	FetchNodePort ContainerComponent = "fetch-node-port"
	Kafka         ContainerComponent = "kafka"
)

// mount
const (
	TlsKeystoreMountPath    = "/zncdata/" + TlsDir
	TlsPkcs12KeyStorePath   = TlsKeystoreMountPath + "/keystore.p12"
	TlsPkcs12TruststorePath = TlsKeystoreMountPath + "/truststore.p12"
	DataMountPath           = "/bitnami/kafka"
	LogMountPath            = "/opt/bitnami/kafka/logs"
	ConfigMountPath         = "/opt/bitnami/kafka/config/server.properties"
	Log4jMountPath          = "/opt/bitnami/kafka/config/log4j.properties"

	NodePortMountPath = "/zncdata/tmp"
	NodePortFileName  = "kafka_nodeport"

	ConfigMapMountPath    = "/configmaps"
	ServerConfigMountPath = "/config"
)

// env

const (
	EnvZookeeperConnections        = "KAFKA_CFG_ZOOKEEPER_CONNECT"
	EnvNode                        = "ENV_NODE"
	EnvNodePort                    = "NODE_PORT"
	EnvPodName                     = "POD_NAME"
	EnvListeners                   = "KAFKA_CFG_LISTENERS"
	EnvAdvertisedListeners         = "KAFKA_CFG_ADVERTISED_LISTENERS"
	EnvListenerSecurityProtocolMap = "KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"
	//ssl
	EnvCertificatePass       = "KAFKA_CERTIFICATE_PASSWORD"
	EnvTlsTrustStoreLocation = "KAFKA_TLS_TRUSTSTORE_FILE"
	EnvTlsType               = "KAFKA_TLS_TYPE"
	EnvTlsEnableFlag         = "KAFKA_TLS_CLIENT_AUTH"
)
