package controller

import (
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Broker), groupName).GenerateResourceName("")
}
