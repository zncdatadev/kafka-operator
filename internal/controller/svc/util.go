package svc

import (
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/util"
)

func CreateGroupServiceName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Broker), groupName).GenerateResourceName("")
}

func CreateClusterServiceName(instanceName string) string {
	return instanceName
}

const (
	GroupServiceClientPort   = 9092
	GroupServiceInternalPort = 19092

	ClusterServiceClientPort     = 9092
	ClusterServiceClientNodePort = 32443

	PodServiceClientPort   = 9092
	PodServiceInternalPort = 19092
)
