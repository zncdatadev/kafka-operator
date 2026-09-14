package controller

import (
	"strings"

	opgoconstant "github.com/zncdatadev/operator-go/pkg/constant"
)

// Writable product directories inside the container. Config is copied from the framework's
// read-only ConfigMap mount (opgoconstant.KubedoopConfigDirMount) into KubedoopConfigDir at
// startup; data is the PVC mount managed by the framework builder.
var (
	KubedoopConfigDir = strings.TrimSuffix(opgoconstant.KubedoopConfigDir, "/")
	KubedoopDataDir   = strings.TrimSuffix(opgoconstant.KubedoopDataDir, "/")
	KubedoopRoot      = strings.TrimSuffix(opgoconstant.KubedoopRoot, "/")
)

const (
	ZookeeperDiscoveryKey = "ZOOKEEPER"
)

// LabelKubernetesInstance is the descriptive instance label key used by the default
// broker anti-affinity selector.
const LabelKubernetesInstance = "app.kubernetes.io/instance"

const (
	EnvJvmArgs              = "EXTRA_ARGS"
	EnvZookeeperConnections = "ZOOKEEPER"
	EnvKafkaLog4jOpts       = "KAFKA_LOG4J_OPTS"
	EnvKafkaHeapOpts        = "KAFKA_HEAP_OPTS"
	EnvNode                 = "NODE"
	EnvPodName              = "POD_NAME"
)
