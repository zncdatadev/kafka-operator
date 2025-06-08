/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/zncdatadev/operator-go/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
)

const (
	Log4jFileName    = "log4j.properties"
	SecurityFileName = "security.properties"
	ServerFileName   = "server.properties"
)

const (
	ClientPortName       = "kafka"
	SecureClientPortName = "kafka-tls"
	InternalPortName     = "internal"
	MetricsPortName      = "metrics"

	ClientPort                = 9092
	SecurityClientPort        = 9093
	InternalPort              = 19092
	SecurityInternalPort      = 19093
	MetricsPort               = 9606
	PodSvcClientNodePortMin   = 30092
	PodSvcInternalNodePortMin = 31092
)

const (
	ImageRepository = "quay.io/zncdatadev/kafka"
	ImageTag        = "3.7.1-kubedoop0.0.0-dev"
	ImagePullPolicy = corev1.PullIfNotPresent

	KubedoopKafkaDataDirName  = "data" // kafka log dirs
	KubedoopLogConfigDirName  = "log-config"
	KubedoopConfigDirName     = "config"
	KubedoopLogDirName        = "log"
	KubedoopListenerBroker    = "listener-broker"
	KubedoopListenerBootstrap = "listener-bootstrap"

	KubedoopRoot                 = "/kubedoop"
	KubedoopDataDir              = KubedoopRoot + "/data"
	KubedoopConfigDir            = KubedoopRoot + "/config"
	KubedoopLogConfigDir         = KubedoopRoot + "/log_config"
	KubedoopLogDir               = KubedoopRoot + "/log"
	KubedoopListenerBrokerDir    = KubedoopRoot + "/listener-broker"
	KubedoopListenerBootstrapDir = KubedoopRoot + "/listener-bootstrap"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KafkaCluster is the Schema for the kafkaclusters API
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaClusterSpec `json:"spec,omitempty"`
	Status status.Status    `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaClusterList contains a list of KafkaCluster
type KafkaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaCluster `json:"items"`
}

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	// +kubebuilder:validation:Optional
	// +default:value={"repo": "quay.io/zncdatadev", "pullPolicy": "IfNotPresent"}
	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Required
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterOperation *commonsv1alpha1.ClusterOperationSpec `json:"clusterOperation,omitempty"`

	// +kubebuilder:validation:Required
	Brokers *BrokersSpec `json:"brokers,omitempty"`
}

type ClusterConfigSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// +kubebuilder:validation:Optional
	Tls *KafkaTlsSpec `json:"tls,omitempty"`

	// +kubebuilder:validation:Optional
	VectorAggregatorConfigMapName string `json:"vectorAggregatorConfigMapName,omitempty"`

	// +kubebuilder:validation:required
	ZookeeperConfigMapName string `json:"zookeeperConfigMapName,omitempty"`
}

type KafkaTlsSpec struct {
	// The SecretClass to use for internal broker communication. Use mutual verification between brokers (mandatory).
	// This setting controls: - Which cert the brokers should use to authenticate themselves against other brokers -
	// Which ca.crt to use when validating the other brokers Defaults to tls
	//
	// +kubebuilder:validation:Optional
	ServerSecretClass string `json:"serverSecretClass,omitempty"`
	// The SecretClass to use for client connections. This setting controls: - If TLS encryption is used at all -
	// Which cert the servers should use to authenticate themselves against the client Defaults to tls.
	//
	// +kubebuilder:validation:Optional
	InternalSecretClass string `json:"internalSecretClass,omitempty"`

	// todo: use secret resource
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="chageit"
	SSLStorePassword string `json:"sslStorePassword,omitempty"`
}

type KafkaAuthenticationSpec struct {
	/*
	 *	 ## TLS provider
	 *
	 *	 Only affects client connections. This setting controls:
	 *	 - If clients need to authenticate themselves against the broker via TLS
	 *	 - Which ca.crt to use when validating the provided client certs
	 *
	 *	 This will override the server TLS settings (if set) in `spec.clusterConfig.tls.serverSecretClass`.
	 */
	// +kubebuilder:validation:Optional
	AuthenticationClass string `json:"authenticationClass,omitempty"`
}

type BrokersSpec struct {
	// +kubebuilder:validation:Optional
	Config *BrokersConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	RoleGroups map[string]*BrokersRoleGroupSpec `json:"roleGroups,omitempty"`

	// +kubebuilder:validation:Optional
	Roleconfig *commonsv1alpha1.RoleConfigSpec `json:"roleconfig,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type BrokersRoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation：Optional
	Config *BrokersConfigSpec `json:"config,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type BrokersConfigSpec struct {
	*commonsv1alpha1.RoleGroupConfigSpec `json:",inline"`

	// The ListenerClass used for connecting to brokers. Should use a direct connection ListenerClass to minimize cost
	// and minimize performance overhead (such as `cluster-internal` or `external-unstable`)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster-internal"
	BrokerListenerClass string `json:"listenerClass,omitempty"`

	// The ListenerClass used for bootstrapping new clients. Should use a stable ListenerClass to avoid unnecessary client restarts (such as `cluster-internal` or `external-stable`).
	// +kubebuilder:validation:Optional
	BootstrapListenerClass string `json:"bootstrapListenerClass,omitempty"`

	// Request secret (currently only autoTls certificates) lifetime from the secret operator, e.g. `7d`, or `30d`.
	// Please note that this can be shortened by the `maxCertificateLifetime` setting on the SecretClass issuing the TLS certificate.
	// +kubebuilder:validation:Optional
	RequestedSecretLifeTime string `json:"requestedSecretLifeTime,omitempty"`
}
type ConfigOverridesSpec struct {
	Server   map[string]string `json:"server.properties,omitempty"`
	Security map[string]string `json:"security.properties,omitempty"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
}
