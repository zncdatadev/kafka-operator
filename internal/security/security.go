package security

import (
	"fmt"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
)

// server.properties keys for the CLIENT listener (TLS without client authentication).
const (
	ClientSSLKeyStoreLocation   = "listener.name.client.ssl.keystore.location"
	ClientSSLKeyStorePassword   = "listener.name.client.ssl.keystore.password"
	ClientSSLKeyStoreType       = "listener.name.client.ssl.keystore.type"
	ClientSSLTrustStoreLocation = "listener.name.client.ssl.truststore.location"
	ClientSSLTrustStorePassword = "listener.name.client.ssl.truststore.password"
	ClientSSLTrustStoreType     = "listener.name.client.ssl.truststore.type"
)

// server.properties keys for the CLIENT_AUTH listener (TLS with client authentication).
const (
	ClientAuthSSLKeyStoreLocation   = "listener.name.client_auth.ssl.keystore.location"
	ClientAuthSSLKeyStorePassword   = "listener.name.client_auth.ssl.keystore.password"
	ClientAuthSSLKeyStoreType       = "listener.name.client_auth.ssl.keystore.type"
	ClientAuthSSLTrustStoreLocation = "listener.name.client_auth.ssl.truststore.location"
	ClientAuthSSLTrustStorePassword = "listener.name.client_auth.ssl.truststore.password"
	ClientAuthSSLTrustStoreType     = "listener.name.client_auth.ssl.truststore.type"
	ClientAuthSSLClientAuth         = "listener.name.client_auth.ssl.client.auth"
)

// server.properties keys for the INTERNAL (broker-to-broker) listener.
const (
	InterBrokerListenerName    = "inter.broker.listener.name"
	InterSSLKeyStoreLocation   = "listener.name.internal.ssl.keystore.location"
	InterSSLKeyStorePassword   = "listener.name.internal.ssl.keystore.password"
	InterSSLKeyStoreType       = "listener.name.internal.ssl.keystore.type"
	InterSSLTrustStoreLocation = "listener.name.internal.ssl.truststore.location"
	InterSSLTrustStorePassword = "listener.name.internal.ssl.truststore.password"
	InterSSLTrustStoreType     = "listener.name.internal.ssl.truststore.type"
	InterSSLClientAuth         = "listener.name.internal.ssl.client.auth"
)

// server.properties keys for the BOOTSTRAP listener (Kerberos).
const (
	BootstrapSSLKeyStoreLocation   = "listener.name.bootstrap.ssl.keystore.location"
	BootstrapSSLKeyStorePassword   = "listener.name.bootstrap.ssl.keystore.password"
	BootstrapSSLKeyStoreType       = "listener.name.bootstrap.ssl.keystore.type"
	BootstrapSSLTrustStoreLocation = "listener.name.bootstrap.ssl.truststore.location"
	BootstrapSSLTrustStorePassword = "listener.name.bootstrap.ssl.truststore.password"
	BootstrapSSLTrustStoreType     = "listener.name.bootstrap.ssl.truststore.type"
)

const PKCS12 = "PKCS12"

// KafkaSecurity resolves the cluster's TLS / Kerberos configuration into port selection,
// server.properties settings and CSI secret volume needs. Volume construction itself is
// delegated to the framework's SecretProvisioner; this type only decides WHAT is needed
// and which config keys reference the mounted paths.
type KafkaSecurity struct {
	KafkaAuthentications        []kafkav1alpha1.KafkaAuthenticationSpec
	ResolvedAnthenticationClass string
	InternalSecretClass         string
	ServerSecretClass           string
	SSLStorePassword            string

	kerberosSecretClass string
}

// NewKafkaSecurity creates a new KafkaSecurity instance from the cluster spec.
func NewKafkaSecurity(cluster *kafkav1alpha1.KafkaCluster) *KafkaSecurity {
	instance := &KafkaSecurity{
		ResolvedAnthenticationClass: "", // AuthenticationClass resolution is not supported yet
	}
	if cluster.Spec.ClusterConfig == nil {
		return instance
	}

	instance.KafkaAuthentications = cluster.Spec.ClusterConfig.Authentication
	if tlsSpec := cluster.Spec.ClusterConfig.Tls; tlsSpec != nil {
		instance.InternalSecretClass = tlsSpec.InternalSecretClass
		instance.ServerSecretClass = tlsSpec.ServerSecretClass
		instance.SSLStorePassword = tlsSpec.SSLStorePassword
	}

	for _, auth := range instance.KafkaAuthentications {
		if auth.Kerberos != nil && auth.Kerberos.KerberosSecretClass != "" {
			instance.kerberosSecretClass = auth.Kerberos.KerberosSecretClass
			break
		}
	}

	return instance
}

// IsKerberosEnabled reports whether Kerberos authentication is configured.
func (k *KafkaSecurity) IsKerberosEnabled() bool {
	return k.kerberosSecretClass != ""
}

// KerberosSecretClass returns the SecretClass providing the Kerberos keytab.
func (k *KafkaSecurity) KerberosSecretClass() string {
	return k.kerberosSecretClass
}

// TlsEnabled checks if TLS encryption is enabled.
func (k *KafkaSecurity) TlsEnabled() bool {
	return k.TlsClientAuthenticationClass() != "" || k.TlsServerSecretClass() != ""
}

// TlsServerSecretClass retrieves an optional TLS secret class for external client -> server communications.
func (k *KafkaSecurity) TlsServerSecretClass() string {
	return k.ServerSecretClass
}

// TlsClientAuthenticationClass retrieves an optional TLS AuthenticationClass.
func (k *KafkaSecurity) TlsClientAuthenticationClass() string {
	return k.ResolvedAnthenticationClass
}

// TlsInternalSecretClass retrieves the internal (broker-to-broker) SecretClass.
func (k *KafkaSecurity) TlsInternalSecretClass() string {
	return k.InternalSecretClass
}

// ClientPort returns the Kafka (secure) client port depending on tls or authentication settings.
func (k *KafkaSecurity) ClientPort() int {
	if k.TlsEnabled() {
		return kafkav1alpha1.SecurityClientPort
	}
	return kafkav1alpha1.ClientPort
}

// BootstrapPort returns the Kafka (secure) bootstrap port.
func (k *KafkaSecurity) BootstrapPort() int {
	if k.TlsEnabled() {
		return kafkav1alpha1.BootstrapSecurePort
	}
	return kafkav1alpha1.BootstrapPort
}

// ClientPortName returns the Kafka (secure) client port name depending on tls or authentication settings.
func (k *KafkaSecurity) ClientPortName() string {
	if k.TlsEnabled() {
		return kafkav1alpha1.SecureClientPortName
	}
	return kafkav1alpha1.ClientPortName
}

// BootstrapPortName returns the bootstrap port name.
func (k *KafkaSecurity) BootstrapPortName() string {
	return kafkav1alpha1.BootstrapPortName
}

// InternalPort returns the Kafka (secure) internal port depending on tls settings.
func (k *KafkaSecurity) InternalPort() int {
	if k.TlsInternalSecretClass() != "" || k.IsKerberosEnabled() {
		return kafkav1alpha1.SecurityInternalPort
	}
	return kafkav1alpha1.InternalPort
}

// ConfigSettings returns required Kafka configuration settings for the server.properties file.
// Keystore/truststore locations are resolved from the provisioner so the config can never
// drift from the actual CSI volume mount paths.
func (k *KafkaSecurity) ConfigSettings(provisioner *opgosecurity.SecretProvisioner) map[string]string {
	config := make(map[string]string)
	// We set either client tls with authentication or client tls without authentication.
	// If authentication is explicitly required we do not want to have any other CAs to
	// be trusted.
	if k.TlsClientAuthenticationClass() != "" {
		serverKeystoreDir := provisioner.MustPath(kafkav1alpha1.TLSKeystoreServerVolumeName)
		config[ClientAuthSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", serverKeystoreDir)
		config[ClientAuthSSLKeyStorePassword] = k.SSLStorePassword
		config[ClientAuthSSLKeyStoreType] = PKCS12
		config[ClientAuthSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", serverKeystoreDir)
		config[ClientAuthSSLTrustStorePassword] = k.SSLStorePassword
		config[ClientAuthSSLTrustStoreType] = PKCS12
		// client auth required
		config[ClientAuthSSLClientAuth] = "required"
	} else if k.TlsServerSecretClass() != "" {
		serverKeystoreDir := provisioner.MustPath(kafkav1alpha1.TLSKeystoreServerVolumeName)
		config[ClientSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", serverKeystoreDir)
		config[ClientSSLKeyStorePassword] = k.SSLStorePassword
		config[ClientSSLKeyStoreType] = PKCS12
		config[ClientSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", serverKeystoreDir)
		config[ClientSSLTrustStorePassword] = k.SSLStorePassword
		config[ClientSSLTrustStoreType] = PKCS12
	}

	if k.IsKerberosEnabled() {
		// The BOOTSTRAP listener reuses the server keystore when TLS is enabled.
		if k.TlsEnabled() {
			serverKeystoreDir := provisioner.MustPath(kafkav1alpha1.TLSKeystoreServerVolumeName)
			config[BootstrapSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", serverKeystoreDir)
			config[BootstrapSSLKeyStorePassword] = k.SSLStorePassword
			config[BootstrapSSLKeyStoreType] = PKCS12
			config[BootstrapSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", serverKeystoreDir)
			config[BootstrapSSLTrustStorePassword] = k.SSLStorePassword
			config[BootstrapSSLTrustStoreType] = PKCS12
		}

		config["sasl.enabled.mechanisms"] = "GSSAPI"
		config["sasl.kerberos.service.name"] = kafkav1alpha1.KerberosServiceName
		config["sasl.mechanism.inter.broker.protocol"] = "GSSAPI"
	}

	// Internal tls
	if k.TlsInternalSecretClass() != "" {
		internalKeystoreDir := provisioner.MustPath(kafkav1alpha1.TLSKeystoreInternalVolumeName)
		config[InterSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", internalKeystoreDir)
		config[InterSSLKeyStorePassword] = k.SSLStorePassword
		config[InterSSLKeyStoreType] = PKCS12
		config[InterSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", internalKeystoreDir)
		config[InterSSLTrustStorePassword] = k.SSLStorePassword
		config[InterSSLTrustStoreType] = PKCS12
		config[InterSSLClientAuth] = "required"
	}
	// common
	config[InterBrokerListenerName] = "INTERNAL"
	return config
}
