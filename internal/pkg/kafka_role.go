package pkg

import (
	"github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

func NewClusterRole(cluster *v1alpha1.KafkaCluster) *KafkaRole {
	return &KafkaRole{
		Cluster: cluster,
	}
}

type KafkaRole struct {
	Cluster *v1alpha1.KafkaCluster
}

func (k *KafkaRole) KerberosServiceName() string {
	return "kafka"
}

// TODO
func (k *KafkaRole) GetKerberosSecretClass() string {
	auths := k.Cluster.Spec.ClusterConfig.Authentication
	for _, auth := range auths {
		if auth.Kerberos != nil && auth.Kerberos.KerberosSecretClass != "" {
			return auth.Kerberos.KerberosSecretClass
		}
	}
	return ""
}
