package container

import (
	"fmt"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	"github.com/zncdata-labs/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
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
		//{
		//	Name:      ServerConfigVolumeName(),
		//	MountPath: common.ConfigMountPath,
		//	SubPath:   kafkav1alpha1.ServerFileName,
		//},
		//{
		//	Name:      Log4jVolumeName(),
		//	MountPath: common.Log4jMountPath,
		//	SubPath:   kafkav1alpha1.Log4jFileName,
		//},
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
kafka_conf_set() {
    local file="${1:?missing file}"
    local key="${2:?missing key}"
    local value="${3:?missing value}"

    # Check if the value was set before
    if grep -q "^[#\\s]*$key\s*=.*" "$file"; then
        # Update the existing key
        replace_in_file "$file" "^[#\\s]*${key}\s*=.*" "${key}=${value}" false
    else
        # Add a new key
        printf '\n%%s=%%s' "$key" "$value" >>"$file"
    fi
}

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

KAFKA_CONFIG_LOCATION=%s
# kafka_conf_set $KAFKA_CONFIG_LOCATION  inter.broker.listener.name INTERNAL

echo "show Envs..."
echo "Node ID: $NODE_ID"
echo "Node Port: $NODE_PORT"
echo "Advertised Listeners: $KAFKA_CFG_ADVERTISED_LISTENERS"

# echo "show Configs..."
# cat $KAFKA_CONFIG_LOCATION

/opt/bitnami/scripts/kafka/entrypoint.sh /opt/bitnami/scripts/kafka/run.sh`, nodePortLoc, advertisedListeners,
		common.ConfigMountPath)
	if common.SslEnabled(d.sslSpec) {
		args += d.setSsl()
	}
	return []string{args}
}

// set ssl properties
func (d *KafkaContainerBuilder) setSsl() string {
	return fmt.Sprintf(`
echo "set ssl properties into kafka server.properties"
SERVER_LOCATION = %s
# key store
kafka_common_conf_set $SERVER_LOCATION ssl.keystore.type %s
kafka_common_conf_set $SERVER_LOCATION ssl.keystore.location %s
kafka_common_conf_set $SERVER_LOCATION ssl.keystore.password %s
# trust store
kafka_common_conf_set $SERVER_LOCATION ssl.truststore.type %s
kafka_common_conf_set $SERVER_LOCATION ssl.truststore.location %s
kafka_common_conf_set $SERVER_LOCATION ssl.truststore.password %s
`, common.ConfigMountPath, d.sslSpec.KeyStoreType, common.TlsPkcs12KeyStorePath, d.sslSpec.KeyStorePassword,
		d.sslSpec.KeyStoreType, common.TlsPkcs12TruststorePath, d.sslSpec.KeyStorePassword)
}

// client advertised
