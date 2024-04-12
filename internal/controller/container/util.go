package container

import "github.com/zncdata-labs/kafka-operator/internal/common"

func DataVolumeName() string {
	return "data"
}

func ConfigVolumeName() string {
	return "config"
}

func Log4jVolumeName() string {
	return "log4j"
}

func Log4jLogVolumeName() string {
	return "log4j-log"
}

func NodePortVolumeName() string {
	return "node-port"
}

const TlsDir = "tls-keystore-internal"

func TlsKeystoreInternalVolumeName() string {
	return "tls-keystore-internal"
}

const (
	FetchNodePort common.ContainerComponent = "fetch-node-port"
	Kafka         common.ContainerComponent = "kafka"
)

// mount
const (
	TlsKeystoreMountPath = "/zncdata/" + TlsDir
	DataMountPath        = "/zncdata/kafka"
	LogMountPath         = "/zncdata/logs"
	ConfigMountPath      = "/opt/bitnami/kafka/config/server.properties"
	Log4jMountPath       = "/opt/bitnami/kafka/config/log4j.properties"

	NodePortMountPath = "/zncdata/tmp"
	NodePortFileName  = "kafka_nodeport"
)

// env

const (
	EnvZookeeperConnections = "ENV_ZOOKEEPER_CONNECTIONS"
	EnvNode                 = "ENV_NODE"
	EnvNodePort             = "NODE_PORT"
	EnvPodName              = "POD_NAME"
)
