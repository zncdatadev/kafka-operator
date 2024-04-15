package common

// Listener csi

const ListenerStorageClass = "listeners.zncdata.dev"
const ListenerAnnotationKey = ListenerStorageClass + "/listener-class"

type ListenerClass string

const (
	// ClusterIp is the default listener class for internal communication
	ClusterIp ListenerClass = "cluster-internal"
	// NodePort is for external communication
	NodePort          ListenerClass = "external-unstable"
	LoadBalancerClass ListenerClass = "external-stable"
)

// secret cis

const SecretStorageClass = "secrets.zncdata.dev"

const SecretAnnotationClass = SecretStorageClass + "/class"
const SecretAnnotationFormat = SecretStorageClass + "/format"
const SecretAnnotationScope = SecretStorageClass + "/scope"
const SecretAnnotationPKCS12Password = SecretStorageClass + "/tlsPKCS12Password"

type SecretClass string

const (
	Tls SecretClass = "tls"
)

type SecretFormat string

const (
	Pem    SecretFormat = "tls-pem"
	Pkcs12 SecretFormat = "tls-p12"
	Jks    SecretFormat = "kerberos"
)

type SecretScope string

const (
	ScopeService        SecretScope = "service"
	ScopeNode           SecretScope = "node"
	ScopeListenerVolume SecretScope = "listener-volume"
	ScopePod            SecretScope = "pod"
)

// Zookeeper

const ZookeeperDiscoveryKey = "ZOOKEEPER"
