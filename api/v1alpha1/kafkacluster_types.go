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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
)

const (
	DefaultRepository     = "quay.io/zncdatadev"
	DefaultProductVersion = "3.9.0"
	DefaultProductName    = "kafka"
)

const (
	Log4jFileName    = "log4j.properties"
	SecurityFileName = "security.properties"
	ServerFileName   = "server.properties"
)

const (
	// BrokerRoleName is the single Kafka role, used as the role key and component label value.
	BrokerRoleName = "broker"

	// KafkaContainerName is the main container name. It is significant: it must match the
	// per-container logging key (logging.containers.kafka) and drives the log file name the
	// Vector sidecar globs (<container>.stdout.log).
	KafkaContainerName = "kafka"

	// KerberosServiceName is the Kerberos service principal primary for Kafka.
	KerberosServiceName = "kafka"
)

const (
	ClientPortName       = "kafka"
	SecureClientPortName = "kafka-tls"
	InternalPortName     = "internal"
	MetricsPortName      = "metrics"
	BootstrapPortName    = "bootstrap"

	ClientPort           = 9092
	SecurityClientPort   = 9093
	InternalPort         = 19092
	SecurityInternalPort = 19093
	MetricsPort          = 9606
	BootstrapPort        = 9094
	BootstrapSecurePort  = 9095
)

const (
	// ListenerBrokerVolumeName is the per-broker listener CSI volume (mounted at
	// /kubedoop/listener/listener-broker). Its name is referenced by the secret-operator
	// scope annotation "listener-volume=listener-broker" on the TLS/Kerberos volumes.
	ListenerBrokerVolumeName = "listener-broker"
	// ListenerBootstrapVolumeName is the bootstrap listener CSI volume (mounted at
	// /kubedoop/listener/listener-bootstrap), referencing the role group bootstrap Listener.
	ListenerBootstrapVolumeName = "listener-bootstrap"

	// KerberosVolumeName is the Kerberos keytab CSI volume (mounted at /kubedoop/mount/kerberos).
	KerberosVolumeName = "kerberos"
	// TLSKeystoreServerVolumeName is the server TLS keystore CSI volume.
	TLSKeystoreServerVolumeName = "tls-keystore-server"
	// TLSKeystoreInternalVolumeName is the internal (broker-to-broker) TLS keystore CSI volume.
	TLSKeystoreInternalVolumeName = "tls-keystore-internal"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KafkaCluster is the Schema for the kafkaclusters API
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaClusterSpec   `json:"spec,omitempty"`
	Status KafkaClusterStatus `json:"status,omitempty"`
}

// KafkaClusterStatus defines the observed state of KafkaCluster.
type KafkaClusterStatus struct {
	commonsv1alpha1.GenericClusterStatus `json:",inline"`
}

// ClusterInterface implementation

// GetSpec adapts the Kafka spec to the framework's GenericClusterSpec.
func (k *KafkaCluster) GetSpec() *commonsv1alpha1.GenericClusterSpec {
	return k.Spec.ToGenericSpec()
}

// GetStatus returns the cluster status.
func (k *KafkaCluster) GetStatus() *commonsv1alpha1.GenericClusterStatus {
	return &k.Status.GenericClusterStatus
}

// VectorAggregatorConfigMapName implements reconciler.VectorAggregatorProvider so the framework
// owns vector.yaml generation: when a role group enables the Vector agent, the GenericReconciler
// resolves the aggregator address from this ConfigMap and renders vector.yaml into the role group
// ConfigMap. Returns "" when unset (the framework then omits vector.yaml).
func (k *KafkaCluster) VectorAggregatorConfigMapName() string {
	if k.Spec.ClusterConfig == nil {
		return ""
	}
	return k.Spec.ClusterConfig.VectorAggregatorConfigMapName
}

// ToGenericSpec adapts KafkaClusterSpec to the framework's GenericClusterSpec:
// brokers -> Roles["broker"].
func (s *KafkaClusterSpec) ToGenericSpec() *commonsv1alpha1.GenericClusterSpec {
	result := &commonsv1alpha1.GenericClusterSpec{
		ClusterOperation: s.ClusterOperation,
	}

	// spec.image passes through untouched: the framework folds it over the handler's
	// ImageDefaults per field at reconcile time (user first), so the repo/version
	// fallbacks live on the handler instead of being normalized into the spec here.
	result.Image = s.Image

	if s.Brokers == nil {
		return result
	}

	roleSpec := commonsv1alpha1.RoleSpec{
		RoleConfig: s.Brokers.Roleconfig,
	}

	if s.Brokers.Config != nil {
		roleSpec.Config = s.Brokers.Config.RoleGroupConfigSpec
	}

	if s.Brokers.OverridesSpec != nil {
		roleSpec.ConfigOverrides = s.Brokers.ConfigOverrides
		roleSpec.EnvOverrides = s.Brokers.EnvOverrides
		roleSpec.CliOverrides = s.Brokers.CliOverrides
		roleSpec.PodOverrides = s.Brokers.PodOverrides
	}

	roleGroups := make(map[string]commonsv1alpha1.RoleGroupSpec)
	for name, rg := range s.Brokers.RoleGroups {
		if rg == nil {
			continue
		}
		roleGroups[name] = adaptRoleGroup(rg)
	}
	roleSpec.RoleGroups = roleGroups

	result.Roles = map[string]commonsv1alpha1.RoleSpec{
		BrokerRoleName: roleSpec,
	}

	return result
}

// adaptRoleGroup converts a Kafka role group spec to the framework's generic shape.
func adaptRoleGroup(rg *BrokersRoleGroupSpec) commonsv1alpha1.RoleGroupSpec {
	adapted := commonsv1alpha1.RoleGroupSpec{}
	// Always carry the stored value: an explicit `replicas: 0` (scale-down) must reach the
	// StatefulSet — mapping it to nil would let the framework default it back to 1. Omitted
	// replicas are defaulted to 1 by the CRD before they ever get here.
	replicas := rg.Replicas
	adapted.Replicas = &replicas
	if rg.Config != nil {
		adapted.Config = rg.Config.RoleGroupConfigSpec
	}
	if rg.OverridesSpec != nil {
		adapted.ConfigOverrides = rg.ConfigOverrides
		adapted.EnvOverrides = rg.EnvOverrides
		adapted.CliOverrides = rg.CliOverrides
		adapted.PodOverrides = rg.PodOverrides
	}
	return adapted
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
	Image *commonsv1alpha1.ImageSpec `json:"image,omitempty"`

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
	Authentication []KafkaAuthenticationSpec `json:"authentication,omitempty"`

	// +kubebuilder:validation:Optional
	Tls *KafkaTlsSpec `json:"tls,omitempty"`

	// +kubebuilder:validation:Optional
	VectorAggregatorConfigMapName string `json:"vectorAggregatorConfigMapName,omitempty"`

	// +kubebuilder:validation:Required
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
	// TODO: Use with operator-go
	AuthenticationClass string `json:"authenticationClass,omitempty"`

	Kerberos *KerberosAuthenticationProviderSpec `json:"kerberos,omitempty"`
}

type KerberosAuthenticationProviderSpec struct {
	KerberosSecretClass string `json:"kerberosSecretClass,omitempty"`
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

	// +kubebuilder:validation:Optional
	Config *BrokersConfigSpec `json:"config,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type BrokersConfigSpec struct {
	*commonsv1alpha1.RoleGroupConfigSpec `json:",inline"`

	// The ListenerClass used for connecting to brokers. Should use a direct connection ListenerClass to minimize cost
	// and minimize performance overhead (such as `cluster-internal` or `external-unstable`).
	// Defaults to `cluster-internal` at consumption time — no CRD default: this block is folded
	// role -> role group, and a structural default here would make any role group declaring
	// `config` silently override the role's value (see operator-go #573/#580).
	// +kubebuilder:validation:Optional
	BrokerListenerClass string `json:"brokerListenerClass,omitempty"`

	// The ListenerClass used for bootstrapping new clients. Should use a stable ListenerClass to avoid unnecessary client restarts (such as `cluster-internal` or `external-stable`).
	// +kubebuilder:validation:Optional
	BootstrapListenerClass string `json:"bootstrapListenerClass,omitempty"`

	// Request secret (currently only autoTls certificates) lifetime from the secret operator, e.g. `7d`, or `30d`.
	// Please note that this can be shortened by the `maxCertificateLifetime` setting on the SecretClass issuing the TLS certificate.
	// +kubebuilder:validation:Optional
	RequestedSecretLifeTime string `json:"requestedSecretLifeTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
}
