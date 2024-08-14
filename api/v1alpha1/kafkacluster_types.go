/*
Copyright 2024.

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
	ImageRepository = "docker.stackable.tech/stackable/kafka"
	ImageTag        = "3.7.1-stackable24.7.0"
	ImagePullPolicy = corev1.PullAlways

	KubedoopKafkaDataDirName = "data" // kafka log dirs
	KubedoopLogConfigDirName = "log-config"
	KubedoopConfigDirName    = "config"
	KubedoopTmpDirName       = "tmp"
	KubedoopLogDirName       = "log"

	KubedoopRoot         = "/stackable"
	KubedoopTmpDir       = KubedoopRoot + "/tmp"
	KubedoopDataDir      = KubedoopRoot + "/data"
	KubedoopConfigDir    = KubedoopRoot + "/config"
	KubedoopLogConfigDir = KubedoopRoot + "/log_config"
	KubedoopLogDir       = KubedoopRoot + "/log"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KafkaCluster is the Schema for the kafkaclusters API
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaClusterSpec `json:"spec,omitempty"`
	Status status.Status    `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KafkaClusterList contains a list of KafkaCluster
type KafkaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaCluster `json:"items"`
}

// KafkaClusterSpec defines the desired state of KafkaCluster
type KafkaClusterSpec struct {
	// +kubebuilder:validation:Optional
	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Required
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Required
	Brokers *BrokersSpec `json:"brokers,omitempty"`
}

type ImageSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="docker.stackable.tech/stackable/kafka"
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="3.7.1-stackable24.7.0"
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type ClusterConfigSpec struct {
	// +kubebuilder:validation:Optional
	Service *ServiceSpec `json:"service,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	Tls *TlsSpec `json:"tls,omitempty"`

	// +kubebuilder:validation:required
	ZookeeperConfigMapName string `json:"zookeeperConfigMapName,omitempty"`
}

type TlsSpec struct {
	// The SecretClass to use for internal broker communication. Use mutual verification between brokers (mandatory).
	// This setting controls: - Which cert the brokers should use to authenticate themselves against other brokers -
	// Which ca.crt to use when validating the other brokers Defaults to tls
	//
	// +kubebuilder:validation:Optional
	ServerSecretClass string `json:"serverSecretClass,omitempty"`
	//The SecretClass to use for client connections. This setting controls: - If TLS encryption is used at all -
	//Which cert the servers should use to authenticate themselves against the client Defaults to tls.
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
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
}

type BrokersRoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Required
	Config *BrokersConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
}

type BrokersConfigSpec struct {
	// +kubebuilder:validation:Optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster-internal"
	ListenerClass string `json:"listenerClass,omitempty"`

	// +kubebuilder:validation:Optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity"`

	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations"`

	// +kubebuilder:validation:Optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// +kubebuilder:validation:Optional
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="8Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// +kubebuilder:validation:Optional
	ExtraEnv map[string]string `json:"extraEnv,omitempty"`

	// +kubebuilder:validation:Optional
	ExtraSecret map[string]string `json:"extraSecret,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *BrokersContainerLoggingSpec `json:"logging,omitempty"`
}
type BrokersContainerLoggingSpec struct {
	// +kubebuilder:validation:Optional
	Broker *LoggingConfigSpec `json:"broker,omitempty"`
}
type ConfigOverridesSpec struct {
	Log4j    map[string]string `json:"log4j.properties,omitempty"`
	Security map[string]string `json:"security.properties,omitempty"`
}

type PodDisruptionBudgetSpec struct {
	// +kubebuilder:validation:Optional
	MinAvailable int32 `json:"minAvailable,omitempty"`

	// +kubebuilder:validation:Optional
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}

type ServiceSpec struct {
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:enum=ClusterIP;NodePort;LoadBalancer;ExternalName
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=18080
	Port int32 `json:"port,omitempty"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
}
