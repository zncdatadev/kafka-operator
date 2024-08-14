package common

const (
	FetchNodePort ContainerComponent = "fetch-node-port"
	Kafka         ContainerComponent = "kafka"
)

// mount
const (
	TlsKeystoreMountPath          = "/bitnami/kafka/config/certs"
	SecretTlsPkcs12KeyStorePath   = TlsKeystoreMountPath + "/keystore.p12"
	SecretTlsPkcs12TrustStorePath = TlsKeystoreMountPath + "/truststore.p12"
	KafkaTlsJksKeyStorePath       = TlsKeystoreMountPath + "/kafka.keystore.jks"
	KafkaTlsJks12TrustStorePath   = TlsKeystoreMountPath + "/kafka.truststore.jks"
	LogMountPath                  = "/opt/bitnami/kafka/logs"
	ConfigMountPath               = "/opt/bitnami/kafka/config/server.properties"
	Log4jMountPath                = "/opt/bitnami/kafka/config/log4j.properties"

	// NodePortMountPath = "/zncdata/tmp"
	NodePortFileName = "kafka_nodeport"
)

// env

const (
	EnvJvmArgs                     = "EXTRA_ARGS"
	EnvZookeeperConnections        = "ZOOKEEPER"
	EnvKafkaLog4jOpts              = "KAFKA_LOG4J_OPTS"
	EnvKafkaHeapOpts               = "KAFKA_HEAP_OPTS"
	EnvNode                        = "NODE"
	EnvNodePort                    = "NODE_PORT"
	EnvPodName                     = "POD_NAME"
	EnvListeners                   = "KAFKA_CFG_LISTENERS"
	EnvAdvertisedListeners         = "KAFKA_CFG_ADVERTISED_LISTENERS"
	EnvListenerSecurityProtocolMap = "KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"
	//ssl
	EnvCertificatePass       = "KAFKA_CERTIFICATE_PASSWORD"
	EnvTlsTrustStoreLocation = "KAFKA_TLS_TRUSTSTORE_FILE"
	EnvTlsType               = "KAFKA_TLS_TYPE"
	EnvKafkaCertDir          = "KAFKA_CERTS_DIR"
)
