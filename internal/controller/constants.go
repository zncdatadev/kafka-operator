package controller

const (
	Kafka  ContainerComponent = "kafka"
	Vector ContainerComponent = "vector"
)

// mount
const (
	ZookeeperDiscoveryKey = "ZOOKEEPER"
	NodePortFileName      = "kafka_nodeport"
)

const (
	EnvJvmArgs              = "EXTRA_ARGS"
	EnvZookeeperConnections = "ZOOKEEPER"
	EnvKafkaLog4jOpts       = "KAFKA_LOG4J_OPTS"
	EnvKafkaHeapOpts        = "KAFKA_HEAP_OPTS"
	EnvNode                 = "NODE"
	EnvNodePort             = "NODE_PORT"
	EnvPodName              = "POD_NAME"
)
