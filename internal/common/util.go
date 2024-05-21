package common

import (
	"fmt"
	"strings"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
)

func OverrideEnvVars(origin *[]corev1.EnvVar, override map[string]string) {
	var originVars = make(map[string]int)
	for i, env := range *origin {
		originVars[env.Name] = i
	}

	for k, v := range override {
		// if env Name is in override, then override it
		if idx, ok := originVars[k]; ok {
			(*origin)[idx].Value = v
		} else {
			// if override's key is new, then append it
			*origin = append(*origin, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
}

func CreateConfigName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(Broker), groupName).GenerateResourceName("")
}

func CreateNetworkUrl(podName string, svcName, namespace, clusterDomain string, port int32) string {
	return podName + "." + CreateDnsDomain(svcName, namespace, clusterDomain, port)
}

func CreateDnsDomain(svcName, namespace, clusterDomain string, port int32) string {
	return fmt.Sprintf("%s:%d", CreateDomainHost(svcName, namespace, clusterDomain), port)
}

func CreateDomainHost(svcName, namespace, clusterDomain string) string {
	return fmt.Sprintf("%s.%s.svc.%s", svcName, namespace, clusterDomain)
}

// CreatePodNamesByReplicas create pod names by replicas
func CreatePodNamesByReplicas(replicas int32, statefulResourceName string) []string {
	podNames := make([]string, replicas)
	for i := int32(0); i < replicas; i++ {
		podName := fmt.Sprintf("%s-%d", statefulResourceName, i)
		podNames[i] = podName
	}
	return podNames
}

func CreateServiceAccountName(instanceName string) string {
	return util.NewResourceNameGeneratorOneRole(instanceName, "").GenerateResourceName("sa")
}

func CreateKvContentByReplicas(replicas int32, keyTemplate string, valueTemplate string) [][2]string {
	var res [][2]string
	for i := int32(0); i < replicas; i++ {
		key := fmt.Sprintf(keyTemplate, i)
		var value string
		if strings.Contains(valueTemplate, "%d") {
			value = fmt.Sprintf(valueTemplate, i)
		} else {
			value = valueTemplate
		}
		res = append(res, [2]string{key, value})
	}
	return res
}

func CreateRoleCfgCacheKey(instanceName string, role Role, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(role), groupName).GenerateResourceName("cache")
}

const (
	DefaultFileAppender = "FILE"
	NoneAppender        = "None"
	ConsoleLogAppender  = "kafkaAppender"
	FileLogAppender     = NoneAppender
)

func CreateLog4jBuilder(containerLogging *kafkav1alpha1.LoggingConfigSpec, consoleAppenderName,
	fileAppenderName string, fileLogLocation string) *Log4jLoggingDataBuilder {
	log4jBuilder := &Log4jLoggingDataBuilder{}
	if loggers := containerLogging.Loggers; loggers != nil {
		var builderLoggers []LogBuilderLoggers
		for logger, level := range loggers {
			builderLoggers = append(builderLoggers, LogBuilderLoggers{
				logger: logger,
				level:  level.Level,
			})
		}
		log4jBuilder.Loggers = builderLoggers
	}
	if console := containerLogging.Console; console != nil {
		log4jBuilder.Console = &LogBuilderAppender{
			appenderName: consoleAppenderName,
			level:        console.Level,
		}
	}
	if file := containerLogging.File; file != nil {
		log4jBuilder.File = &LogBuilderAppender{
			appenderName:       fileAppenderName,
			level:              file.Level,
			defaultLogLocation: fileLogLocation,
		}
	}

	return log4jBuilder
}
func CreateGroupServiceName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(Broker), groupName).GenerateResourceName("")
}

type ListenerName string

const (
	CLIENT   ListenerName = "CLIENT"
	INTERNAL ListenerName = "INTERNAL"
)

func CreateListener(listenerName ListenerName, host string, port string) string {
	return fmt.Sprintf("%s://%s:%s", listenerName, host, port)
}

func K8sEnvRef(envName string) string {
	return fmt.Sprintf("$(%s)", envName)
}

func LinuxEnvRef(envName string) string {
	return fmt.Sprintf("$%s", envName)
}

func SslEnabled(sslSpec *kafkav1alpha1.SslSpec) bool {
	return sslSpec != nil && sslSpec.Enabled
}
