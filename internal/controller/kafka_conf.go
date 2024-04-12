package controller

import (
	"fmt"
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/controller/container"
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
log4j.appender.FILE.File=/zncdata/log/kafka/kafka.log
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
	sslSpec      *v1alpha1.SslSpec
	svcDnsDomain string
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
		k.listeners() + "\n" +
		k.interBrokerListenerName() + "\n" +
		k.advertisedListeners() + "\n" +
		k.listenerSslProperties(k.sslSpec) + "\n" +
		k.dataDir() + "\n" +
		k.zkConnections() + "\n" +
		k.listenerSecurityProtocolMap(k.sslSpec)
}

func (k *KafkaConfGenerator) FileName() string {
	return v1alpha1.ServerFileName
}

type ListenerName string

const (
	CLIENT   ListenerName = "CLIENT"
	INTERNAL ListenerName = "INTERNAL"
)

const interListerName = INTERNAL

// inter broker listener name
func (k *KafkaConfGenerator) interBrokerListenerName() string {
	return fmt.Sprintf("inter.broker.listener.name=%s", interListerName)
}

// listener prefix
func (k *KafkaConfGenerator) listenerPrefix(listenerName ListenerName) string {
	return fmt.Sprintf("listener.name.%s.", strings.ToLower(string(listenerName)))
}

// listeners
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listeners=CLIENT://:9092,ClUSTER://:19092,EXTERNAL://:{nodePort}
func (k *KafkaConfGenerator) listeners() string {
	return fmt.Sprintf("listeners=%s://:%d,%s://:%d", CLIENT, v1alpha1.KafkaClientPort,
		INTERNAL, v1alpha1.InternalPort)
}

// advertised.listeners
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// advertised.listeners=CLIENT://${env.NODE}:${env.NODE_PORT},INTERNAL://${env.POD_NAME}.simple-kafka-broker-default.default.svc.cluster.local:19093
func (k *KafkaConfGenerator) advertisedListeners() string {
	url := fmt.Sprintf("${env.%s}.%s", container.EnvPodName, k.svcDnsDomain)
	return fmt.Sprintf("advertised.listeners=%s://${env.%s}:${env.%s},%s://%s:%d",
		CLIENT, container.EnvNode, container.EnvNodePort, INTERNAL, url, v1alpha1.InternalPort)
}

// listenerSecurityProtocolMap
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listener.security.protocol.map=CLIENT:PLAINTEXT,CLUSTER:PLAINTEXT,EXTERNAL:PLAINTEXT
func (k *KafkaConfGenerator) listenerSecurityProtocolMap(sslSpec *v1alpha1.SslSpec) string {
	var protocol common.SecurityProtocol
	if k.sslEnabled(sslSpec) {
		protocol = common.Ssl
	} else {
		protocol = common.Plaintext
	}
	return fmt.Sprintf("listener.security.protocol.map=%s:PLAINTEXT,%s:%s", CLIENT, INTERNAL, string(protocol))
}

// data dir
// ex: log.dirs=/var/lib/zookeeper/data
func (k *KafkaConfGenerator) dataDir() string {
	return fmt.Sprintf("log.dirs=%s", container.DataMountPath+"/data/topicdata")
}

// zk connections

// zk connections
// ex: zookeeper.connect=zookeeper-service:2181
func (k *KafkaConfGenerator) zkConnections() string {
	return fmt.Sprintf("zookeeper.connect=${env." + container.EnvZookeeperConnections + "}")
}

// listener ssl properties
// todo: wait for secret csi
// listener ssl properties
// ssl.keystore.location=/opt/kafka/config/keystore.jks
// ssl.keystore.password=${env.KAFKA_SSL_KEYSTORE_PASSWORD}
// ssl.key.password=${env.KAFKA_SSL_KEY_PASSWORD}
// ssl.truststore.location=/opt/kafka/config/truststore.jks
// ssl.truststore.password=${env.KAFKA_SSL_TRUSTSTORE_PASSWORD}
func (k *KafkaConfGenerator) listenerSslProperties(sslSpec *v1alpha1.SslSpec) string {
	if k.sslEnabled(sslSpec) {
		return fmt.Sprintf(k.listenerPrefix(interListerName) + ".ssl.keystore.location=" + container.TlsKeystoreMountPath + "/keystore.jks\n" +
			k.listenerPrefix(interListerName) + "ssl.keystore.password=" + sslSpec.KeyStorePassword + "\n" +
			k.listenerPrefix(interListerName) + "ssk.keystore.type=" + sslSpec.KeyStoreType + "\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.location=" + container.TlsKeystoreMountPath + "/truststore.jks\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.password=" + sslSpec.TrustStorePassword + "\n" +
			k.listenerPrefix(interListerName) + "ssl.truststore.type=" + sslSpec.TrustStoreType)
	}
	return ""
}

// ssl enabled
func (k *KafkaConfGenerator) sslEnabled(sslSpec *v1alpha1.SslSpec) bool {
	return sslSpec != nil && sslSpec.Ssl == string(v1alpha1.SslPolicyRequired)
}
