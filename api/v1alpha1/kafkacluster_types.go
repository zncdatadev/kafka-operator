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
	"github.com/zncdata-labs/operator-go/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ClusterConfigSpec *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Required
	Brokers *BrokersSpec `json:"brokers,omitempty"`
}

type ImageSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=docker.stackable.tech/stackable/hadoop
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="3.3.4-stackable24.3.0"
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

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	DfsReplication int32 `json:"dfsReplication,omitempty"`

	// +kubebuilder:validation:required
	ZookeeperDiscoveryZNode string `json:"zookeeperDiscoveryZNode,omitempty"`
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
	Server   map[string]string `json:"server.properties,omitempty"`
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
