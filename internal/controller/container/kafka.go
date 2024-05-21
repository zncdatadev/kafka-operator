package container

import (
	"fmt"
	"strconv"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	"github.com/zncdatadev/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type KafkaContainerBuilder struct {
	common.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *kafkav1alpha1.ResourcesSpec
	sslSpec                 *kafkav1alpha1.SslSpec
	svcNetworkUrl           string
}

func NewKafkaContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *kafkav1alpha1.ResourcesSpec,
	sslSpec *kafkav1alpha1.SslSpec,
	svcNetworkUrl string,
) *KafkaContainerBuilder {
	return &KafkaContainerBuilder{
		ContainerBuilder:        *common.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		sslSpec:                 sslSpec,
		svcNetworkUrl:           svcNetworkUrl,
	}
}

func (d *KafkaContainerBuilder) ContainerName() string {
	return string(common.Kafka)
}

func (d *KafkaContainerBuilder) ContainerEnv() []corev1.EnvVar {
	kafkaCfgGenerator := common.KafkaConfGenerator{
		SslSpec: d.sslSpec,
	}
	envs := []corev1.EnvVar{
		{
			Name: common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: common.EnvNode,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: common.EnvZookeeperConnections,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: d.zookeeperDiscoveryZNode,
					},
					Key: common.ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  common.EnvListeners,
			Value: kafkaCfgGenerator.Listeners(),
		},
		{
			Name:  common.EnvListenerSecurityProtocolMap,
			Value: kafkaCfgGenerator.ListenerSecurityProtocolMap(d.sslSpec),
		},
		// tls
		{
			Name:  common.EnvKafkaCertDir,
			Value: common.TlsKeystoreMountPath,
		},
		// bitnami can not support pkcs12 format, only pem or jks
		//{
		//	Name:  common.EnvTlsType,
		//	Value: d.sslSpec.StoreType,
		//},
		{
			Name:  common.EnvCertificatePass,
			Value: d.sslSpec.JksPassword,
		},
		// export in container command args script
		//{
		//	Name: EnvAdvertisedListeners,
		//},
	}
	//if common.SslEnabled(d.sslSpec) {
	//	envs = append(envs, []corev1.EnvVar{}...)
	//}
	return envs
}

// create advertised listeners

func (d *KafkaContainerBuilder) advertisedListeners() string {
	adClient := common.CreateListener(common.CLIENT, common.LinuxEnvRef(common.EnvNode), common.LinuxEnvRef(common.EnvNodePort))
	internalHost := common.LinuxEnvRef(common.EnvPodName) + "." + d.svcNetworkUrl
	adInternal := common.CreateListener(common.INTERNAL, internalHost, strconv.Itoa(kafkav1alpha1.InternalPort))
	return fmt.Sprintf("%s,%s", adClient, adInternal)
}

// create listener

func (d *KafkaContainerBuilder) ResourceRequirements() corev1.ResourceRequirements {
	return *util.ConvertToResourceRequirements(d.resourceSpec)
}

func (d *KafkaContainerBuilder) VolumeMount() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      NodePortVolumeName(),
			MountPath: common.NodePortMountPath,
		},
		{
			Name:      DataVolumeName(),
			MountPath: common.DataMountPath,
		},
		{
			Name:      Log4jVolumeName(),
			MountPath: common.Log4jMountPath,
			SubPath:   kafkav1alpha1.Log4jFileName,
		},
		{
			Name:      Log4jLoggingVolumeName(),
			MountPath: common.LogMountPath,
		},
	}
	if common.SslEnabled(d.sslSpec) {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      common.TlsKeystoreInternalVolumeName(),
			MountPath: common.TlsKeystoreMountPath,
		})
	}
	return mounts
}

func (d *KafkaContainerBuilder) LivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    6,
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      5,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkav1alpha1.KafkaPortName)},
		},
	}
}

func (d *KafkaContainerBuilder) ReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkav1alpha1.KafkaPortName)},
		},
	}
}

// ContainerPorts  make container ports of data node
func (d *KafkaContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          kafkav1alpha1.KafkaPortName,
			ContainerPort: kafkav1alpha1.KafkaClientPort,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          kafkav1alpha1.InternalPortName,
			ContainerPort: kafkav1alpha1.InternalPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func (d *KafkaContainerBuilder) Command() []string {
	return []string{"sh", "-c"}
}

// CommandArgs command args
// ex: export NODE_PORT=$(cat /zncdata/tmp/kafka_nodepor
func (d *KafkaContainerBuilder) CommandArgs() []string {
	nodePortLoc := common.NodePortMountPath + "/" + common.NodePortFileName
	advertisedListeners := d.advertisedListeners()
	args := fmt.Sprintf(`
echo "Setting up kafka"

NODE_PORT_LOCATION=%s
if [ -f $NODE_PORT_LOCATION ]; then
    NODE_PORT=$(cat $NODE_PORT_LOCATION)
    if [ -z "$NODE_PORT" ]; then
        echo "No NodePort found in $NODE_PORT_LOCATION" >&2
        exit 1
    fi
else
    echo "$NODE_PORT_LOCATION does not exist" >&2
    exit 1
fi

export KAFKA_CFG_ADVERTISED_LISTENERS=%s
export NODE_ID=${POD_NAME##*-}
export KAFKA_CFG_NODE_ID=${NODE_ID}
export KAFKA_CFG_INTER_BROKER_LISTENER_NAME=INTERNAL

echo "show Envs..."
echo "Node ID: $NODE_ID"
echo "Node Port: $NODE_PORT"
echo "Advertised Listeners: $KAFKA_CFG_ADVERTISED_LISTENERS"

/opt/bitnami/scripts/kafka/entrypoint.sh /opt/bitnami/scripts/kafka/run.sh`, nodePortLoc, advertisedListeners)
	if common.SslEnabled(d.sslSpec) {
		args = d.transformPkcs12ToJks() + d.setSsl() + args
	}
	return []string{args}
}

// set ssl properties
func (d *KafkaContainerBuilder) setSsl() string {
	return fmt.Sprintf(`
echo "set ssl envs..."
# key store
export KAFKA_CFG_SSL_KEYSTORE_LOCATION=%s
export KAFKA_CFG_SSL_KEYSTORE_TYPE=%s
export KAFKA_CFG_SSL_KEY_PASSWORD=%s
# trust store
export KAFKA_CFG_SSL_TRUSTSTORE_LOCATION=%s
export KAFKA_CFG_SSL_TRUSTSTORE_TYPE=%s
export KAFKA_CFG_SSL_TRUSTSTORE_PASSWORD=%s
`,
		common.KafkaTlsJksKeyStorePath, common.SslJks, d.sslSpec.JksPassword,
		common.KafkaTlsJks12TrustStorePath, common.SslJks, d.sslSpec.JksPassword)
}

// transform keystore and truststore file from pcks12 to jks using the same password
// First determine whether the key file is in jks format, if it is, transform it to jks format,
// if it is jks format, do nothing
// if it is other,echo unexpected error
func (d *KafkaContainerBuilder) transformPkcs12ToJks() string {
	return fmt.Sprintf(`
set -ex

echo "transform keystore and truststore file from pcks12 to jks..."
SECRET_FORMAT=%s
SSL_STORE_PASSWORD=%s
CERT_DIR=%s
if [ "$SECRET_FORMAT" = "JKS" ]; then
    echo "already in jks format"
    exit 0
fi
if [ "$SECRET_FORMAT" = "PKCS12" ]; then
    echo "transforming pkcs12 to jks format"
    keytool -importkeystore -srckeystore "${CERT_DIR}/keystore.p12" -destkeystore "${CERT_DIR}/kafka.keystore.jks" -srcstoretype pkcs12 -deststoretype jks -srcstorepass $SSL_STORE_PASSWORD -deststorepass $SSL_STORE_PASSWORD  -noprompt
    keytool -importkeystore -srckeystore "${CERT_DIR}/truststore.p12" -destkeystore "${CERT_DIR}/kafka.truststore.jks" -srcstoretype pkcs12 -deststoretype jks -srcstorepass $SSL_STORE_PASSWORD -deststorepass $SSL_STORE_PASSWORD -noprompt
else
    echo "unsupported secret format: $SECRET_FORMAT"
    exit 1
fi
`, "PKCS12", d.sslSpec.JksPassword, common.TlsKeystoreMountPath)

}
