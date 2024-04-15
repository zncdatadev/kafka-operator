package controller

import (
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Broker), groupName).GenerateResourceName("")
}

func sslEnabled(sslSpec *v1alpha1.SslSpec) bool {
	return sslSpec != nil && sslSpec.Enabled == string(v1alpha1.SslPolicyRequired)
}
