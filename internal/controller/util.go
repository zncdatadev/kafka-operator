package controller

import (
	"github.com/zncdata-labs/kafka-operator/internal/util"
)

func createConfigName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(Broker), groupName).GenerateResourceName("")
}

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(Broker), groupName).GenerateResourceName("")
}

func createServiceName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(Broker), groupName).GenerateResourceName("")
}

const (
	ServiceHttpPort   = 8480
	ServiceRpcPort    = 8485
	ServiceMetricPort = 8081
)
