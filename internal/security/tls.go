package security

import (
	"fmt"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/util"
	"github.com/zncdatadev/operator-go/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Client
const (
	ClientSSLKeyStoreLocation   = "listener.name.client.ssl.keystore.location"
	ClientSSLKeyStorePassword   = "listener.name.client.ssl.keystore.password"
	ClientSSLKeyStoreType       = "listener.name.client.ssl.keystore.type"
	ClientSSLTrustStoreLocation = "listener.name.client.ssl.truststore.location"
	ClientSSLTrustStorePassword = "listener.name.client.ssl.truststore.password"
	ClientSSLTrustStoreType     = "listener.name.client.ssl.truststore.type"
)

// ClientAuth
const (
	ClientAuthSSLKeyStoreLocation   = "listener.name.client_auth.ssl.keystore.location"
	ClientAuthSSLKeyStorePassword   = "listener.name.client_auth.ssl.keystore.password"
	ClientAuthSSLKeyStoreType       = "listener.name.client_auth.ssl.keystore.type"
	ClientAuthSSLTrustStoreLocation = "listener.name.client_auth.ssl.truststore.location"
	ClientAuthSSLTrustStorePassword = "listener.name.client_auth.ssl.truststore.password"
	ClientAuthSSLTrustStoreType     = "listener.name.client_auth.ssl.truststore.type"
	ClientAuthSSLClientAuth         = "listener.name.client_auth.ssl.client.auth"
)

// Internal
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

// Directories
const (
	KubedoopTLSCertServerDir           = kafkav1alpha1.KubedoopRoot + "/tls_cert_server_mount"
	KubedoopTLSCertServerDirName       = "tls-cert-server-mount"
	KubedoopTLSKeyStoreServerDir       = kafkav1alpha1.KubedoopRoot + "/tls_keystore_server"
	KubedoopTLSKeyStoreServerDirName   = "tls-keystore-server"
	KubedoopTLSKeyStoreInternalDir     = kafkav1alpha1.KubedoopRoot + "/tls_keystore_internal"
	KubedoopTLSKeyStoreInternalDirName = "tls-keystore-internal"
)

type KafkaTlsSecurity struct {
	ResolvedAnthenticationClass string
	InternalSecretClass         string
	ServerSecretClass           string
	SSLStorePassword            string
}

// NewKafkaTlsSecurity creates a new KafkaTlsSecurity instance
func NewKafkaTlsSecurity(tlsSpec *kafkav1alpha1.TlsSpec) *KafkaTlsSecurity {
	if tlsSpec == nil {
		return &KafkaTlsSecurity{}
	}
	return &KafkaTlsSecurity{
		ResolvedAnthenticationClass: "", // unsupport currently
		InternalSecretClass:         tlsSpec.InternalSecretClass,
		ServerSecretClass:           tlsSpec.ServerSecretClass,
		SSLStorePassword:            tlsSpec.SSLStorePassword,
	}
}

// TlsEnabled checks if TLS encryption is enabled
func (k *KafkaTlsSecurity) TlsEnabled() bool {
	return k.TlsClientAuthenticationClass() != "" || k.TlsServerSecretClass() != ""
}

// TlsServerSecretClass retrieves an optional TLS secret class for external client -> server communications
func (k *KafkaTlsSecurity) TlsServerSecretClass() string {
	return k.ServerSecretClass
}

// TlsClientAuthenticationClass retrieves an optional TLS AuthenticationClass
func (k *KafkaTlsSecurity) TlsClientAuthenticationClass() string {
	return k.ResolvedAnthenticationClass
}

// TlsInternalSecretClass retrieves the mandatory internal SecretClass
func (k *KafkaTlsSecurity) TlsInternalSecretClass() string {
	if k.InternalSecretClass != "" {
		return k.InternalSecretClass
	}
	return ""
}

// ClientPort returns the Kafka (secure) client port depending on tls or authentication settings
func (k *KafkaTlsSecurity) ClientPort() int {
	if k.TlsEnabled() {
		return kafkav1alpha1.SecurityClientPort
	}
	return kafkav1alpha1.ClientPort
}

// ClientPortName returns the Kafka (secure) client port name depending on tls or authentication settings
func (k *KafkaTlsSecurity) ClientPortName() string {
	if k.TlsEnabled() {
		return kafkav1alpha1.SecureClientPortName
	}
	return kafkav1alpha1.ClientPortName
}

// InternalPort returns the Kafka (secure) internal port depending on tls settings
func (k *KafkaTlsSecurity) InternalPort() int {
	if k.TlsInternalSecretClass() != "" {
		return kafkav1alpha1.SecurityInternalPort
	}
	return kafkav1alpha1.InternalPort
}

// SvcContainerCommands returns SVC container command to retrieve the node port service port
func (k *KafkaTlsSecurity) SvcContainerCommands() string {
	portName := k.ClientPortName()
	return fmt.Sprintf("kubectl get service \"$POD_NAME\" -o jsonpath='{.spec.ports[?(@.name==\"%s\")].nodePort}' | tee %s/%s_nodeport", portName, "/tmp", portName)
}

// KcatProberContainerCommands returns the commands for the kcat readiness probe
func (k *KafkaTlsSecurity) KcatProberContainerCommands() []string {
	args := []string{kafkav1alpha1.KubedoopRoot + "/kcat"}
	port := k.ClientPort()

	if k.TlsClientAuthenticationClass() != "" {
		args = append(args, "-b", fmt.Sprintf("localhost:%d", port))
		args = append(args, k.KcatClientAuthSsl(KubedoopTLSCertServerDir)...)
	} else if k.TlsServerSecretClass() != "" {
		args = append(args, "-b", fmt.Sprintf("localhost:%d", port))
		args = append(args, k.KcatClientSsl(KubedoopTLSCertServerDir)...)
	} else {
		args = append(args, "-b", fmt.Sprintf("localhost:%d", port))
	}

	args = append(args, "-L")
	return args
}

// KcatClientAuthSsl returns the SSL configuration for kcat client with authentication
func (k *KafkaTlsSecurity) KcatClientAuthSsl(certDirectory string) []string {
	return []string{
		"-X", "security.protocol=SSL",
		"-X", fmt.Sprintf("ssl.key.location=%s/tls.key", certDirectory),
		"-X", fmt.Sprintf("ssl.certificate.location=%s/tls.crt", certDirectory),
		"-X", fmt.Sprintf("ssl.ca.location=%s/ca.crt", certDirectory),
	}
}

// KcatClientSsl returns the SSL configuration for kcat client
func (k *KafkaTlsSecurity) KcatClientSsl(certDirectory string) []string {
	return []string{
		"-X", "security.protocol=SSL",
		"-X", fmt.Sprintf("ssl.ca.location=%s/ca.crt", certDirectory),
	}
}

// AddVolumeAndVolumeMounts adds required volumes and volume mounts to the pod and container builders
func (k *KafkaTlsSecurity) AddVolumeAndVolumeMounts(sts *appsv1.StatefulSet) {
	kafkaContainer := k.getContainer(sts.Spec.Template.Spec.Containers, "kafka")
	if tlsServerSecretClass := k.TlsServerSecretClass(); tlsServerSecretClass != "" {
		k.AddVolume(sts, CreateTlsVolume(KubedoopTLSCertServerDirName, tlsServerSecretClass, k.SSLStorePassword))
		// cbKcatProber.AddVolumeMount(KubedoopTLSCertServerDirName, KubedoopTLSCertServerDir) todo
		k.AddVolume(sts, CreateTlsKeystoreVolume(KubedoopTLSKeyStoreServerDirName, tlsServerSecretClass, k.SSLStorePassword))
		k.AddVolumeMount(kafkaContainer, KubedoopTLSKeyStoreServerDirName, KubedoopTLSKeyStoreServerDir)
	}

	if tlsInternalSecretClass := k.TlsInternalSecretClass(); tlsInternalSecretClass != "" {
		k.AddVolume(sts, CreateTlsKeystoreVolume(KubedoopTLSKeyStoreInternalDirName, tlsInternalSecretClass, k.SSLStorePassword))
		k.AddVolumeMount(kafkaContainer, KubedoopTLSKeyStoreInternalDirName, KubedoopTLSKeyStoreInternalDir)
	}
}

// statefulset add tls volumes
func (k *KafkaTlsSecurity) AddVolume(sts *appsv1.StatefulSet, volume corev1.Volume) {
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, volume)
}

// container add tls volume mount
func (k *KafkaTlsSecurity) AddVolumeMount(container *corev1.Container, volumeName, mountPath string) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: volumeName, MountPath: mountPath})
}

// get the container by container name in containers
func (k *KafkaTlsSecurity) getContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

// ConfigSettings returns required Kafka configuration settings for the server.properties file
func (k *KafkaTlsSecurity) ConfigSettings() map[string]string {
	config := make(map[string]string)
	// We set either client tls with authentication or client tls without authentication
	// If authentication is explicitly required we do not want to have any other CAs to
	// be trusted.
	if k.TlsClientAuthenticationClass() != "" {
		config[ClientAuthSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", KubedoopTLSKeyStoreServerDir)
		config[ClientAuthSSLKeyStorePassword] = k.SSLStorePassword
		config[ClientAuthSSLKeyStoreType] = "PKCS12"
		config[ClientAuthSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", KubedoopTLSKeyStoreServerDir)
		config[ClientAuthSSLTrustStorePassword] = k.SSLStorePassword
		config[ClientAuthSSLTrustStoreType] = "PKCS12"
		// client auth required
		config[ClientAuthSSLClientAuth] = "required"
	} else if k.TlsServerSecretClass() != "" {
		config[ClientSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", KubedoopTLSKeyStoreServerDir)
		config[ClientSSLKeyStorePassword] = k.SSLStorePassword
		config[ClientSSLKeyStoreType] = "PKCS12"
		config[ClientSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", KubedoopTLSKeyStoreServerDir)
		config[ClientSSLTrustStorePassword] = k.SSLStorePassword
		config[ClientSSLTrustStoreType] = "PKCS12"
	}
	// Internal tls
	if k.TlsInternalSecretClass() != "" {
		config[InterSSLKeyStoreLocation] = fmt.Sprintf("%s/keystore.p12", KubedoopTLSKeyStoreInternalDir)
		config[InterSSLKeyStorePassword] = k.SSLStorePassword
		config[InterSSLKeyStoreType] = "PKCS12"
		config[InterSSLTrustStoreLocation] = fmt.Sprintf("%s/truststore.p12", KubedoopTLSKeyStoreInternalDir)
		config[InterSSLTrustStorePassword] = k.SSLStorePassword
		config[InterSSLTrustStoreType] = "PKCS12"
		config[InterSSLClientAuth] = "required"
	}
	// common
	config[InterBrokerListenerName] = "internal"
	return config
}

func CreateTlsVolume(volumeName, secretClass, sslStorePassword string) corev1.Volume {
	builder := util.SecretVolumeBuilder{VolumeName: volumeName}
	builder.SetAnnotations(map[string]string{
		constants.AnnotationSecretsClass: secretClass,
		constants.AnnotationSecretsScope: fmt.Sprintf("%s,%s", constants.PodScope, constants.NodeScope),
	})
	if sslStorePassword != "" {
		builder.AddAnnotation(constants.AnnotationSecretsPKCS12Password, sslStorePassword)
	}
	return builder.Build()
}

// // CreateTlsKeystoreVolume creates ephemeral volumes to mount the SecretClass into the Pods as keystores
func CreateTlsKeystoreVolume(volumeName, secretClass, sslStorePassword string) corev1.Volume {
	builder := util.SecretVolumeBuilder{VolumeName: volumeName}
	builder.SetAnnotations(map[string]string{
		constants.AnnotationSecretsClass:  secretClass,
		constants.AnnotationSecretsScope:  fmt.Sprintf("%s,%s", constants.PodScope, constants.NodeScope),
		constants.AnnotationSecretsFormat: string(constants.TLSP12),
	})
	if sslStorePassword != "" {
		builder.AddAnnotation(constants.AnnotationSecretsPKCS12Password, sslStorePassword)
	}
	return builder.Build()
}
