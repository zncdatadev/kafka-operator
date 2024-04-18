package container

import (
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
)

func DataVolumeName() string {
	return "data"
}

func Log4jVolumeName() string {
	return "log4j-config"
}

func NodePortVolumeName() string {
	return "node-port"
}

func Log4jLoggingVolumeName() string {
	return "log4j-logging"
}

func ConfigmapVolumeName() string {
	return "configmap"
}
func ServerConfigVolumeName() string {
	return "kafka-config"
}

func sslEnabled(sslSpec *v1alpha1.SslSpec) bool {
	return sslSpec != nil && sslSpec.Enabled == string(v1alpha1.SslPolicyRequired)
}
