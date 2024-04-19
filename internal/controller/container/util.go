package container

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
