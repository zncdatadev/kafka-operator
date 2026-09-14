package controller

import (
	"fmt"
	"strings"
	"time"

	opgoconstant "github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/listener"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

// defaultStorageCapacity is the fallback data PVC size when resources.storage is not
// specified, mirroring the pre-framework default so a minimal KafkaCluster still gets
// persistent topic data storage.
const defaultStorageCapacity = "2Gi"

// defaultGracefulShutdownTimeout mirrors the pre-framework default termination grace.
const defaultGracefulShutdownTimeout = 30 * time.Second

// Pre-framework broker resource defaults: they guarantee requests/limits (and thus the
// KAFKA_HEAP_OPTS derived from the memory limit) even for a minimal KafkaCluster.
const (
	defaultCPURequest  = "250m"
	defaultCPULimit    = "1000m"
	defaultMemoryLimit = "1Gi"
)

// customizeStatefulSet applies the Kafka specifics that vary per role group to the
// StatefulSet built by the base handler: the start args (with the listener overrides) and
// env. Everything role-wide — command interpreter, TCP probes, data PVC, resource/affinity/
// grace defaults — is declared in DeclareRoles; pod identity (ServiceAccount), the default
// pod/container SecurityContext, the config ConfigMap mount, the shared Vector log volume
// and the CSI volumes (TLS/Kerberos/listeners, registered via buildCtx.VolumeProviders)
// are already in place from the framework builder.
func (h *KafkaRoleGroupHandler) customizeStatefulSet(
	sts *appsv1.StatefulSet,
	buildCtx *reconciler.RoleGroupBuildContext,
	cr *kafkav1alpha1.KafkaCluster,
	kafkaSecurity *security.KafkaSecurity,
	secretProvisioner *opgosecurity.SecretProvisioner,
	listenerProvisioner *listener.ListenerProvisioner,
) error {
	podSpec := &sts.Spec.Template.Spec

	if len(podSpec.Containers) == 0 {
		return fmt.Errorf("base handler produced no main container")
	}

	// The framework renamed the primary container to "kafka"
	// (BaseRoleGroupHandler.MainContainerName) and gave it the framework-managed
	// config/data mounts plus the registered CSI volume mounts, so we only set the
	// command, env and probes here. The builder applied the user's podOverrides BEFORE this
	// customization runs, so every field is set only when the user's override container did
	// not set it — user podOverrides keep precedence (pre-framework behavior).
	main := &podSpec.Containers[0]
	override := podOverrideContainer(buildCtx, main.Name)
	if len(override.Args) == 0 {
		args, err := h.getMainContainerArgs(buildCtx, kafkaSecurity, secretProvisioner, listenerProvisioner)
		if err != nil {
			return err
		}
		main.Args = args
	}
	// User envOverrides (already on the container from the builder) win over our defaults.
	main.Env = append(h.getEnvVars(cr, kafkaSecurity, secretProvisioner), main.Env...)

	return nil
}

// podOverrideContainer returns the user's podOverrides entry for the named container (an
// empty container when there is none), so customization can yield to user-set fields.
func podOverrideContainer(buildCtx *reconciler.RoleGroupBuildContext, name string) corev1.Container {
	if buildCtx.MergedConfig == nil || buildCtx.MergedConfig.PodOverrides == nil {
		return corev1.Container{}
	}
	for _, c := range buildCtx.MergedConfig.PodOverrides.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	return corev1.Container{}
}

// getMainContainerArgs returns the shell script the main container runs: copy the
// read-only config mount into the writable config dir, resolve the Kerberos realm when
// enabled, then exec the broker with the computed listener overrides so the JVM receives
// SIGTERM directly for graceful shutdown (Vector shutdown ordering is handled by the
// framework's native sidecar).
func (h *KafkaRoleGroupHandler) getMainContainerArgs(
	buildCtx *reconciler.RoleGroupBuildContext,
	kafkaSecurity *security.KafkaSecurity,
	secretProvisioner *opgosecurity.SecretProvisioner,
	listenerProvisioner *listener.ListenerProvisioner,
) ([]string, error) {
	listenerConfig, err := GetKafkaListenerConfig(buildCtx, kafkaSecurity, listenerProvisioner)
	if err != nil {
		return nil, fmt.Errorf("failed to compute listener config: %w", err)
	}

	launch := fmt.Sprintf(
		`exec bin/kafka-server-start.sh %s/%s --override "zookeeper.connect=${ZOOKEEPER}" --override "listeners=%s" --override "advertised.listeners=%s" --override "listener.security.protocol.map=%s"`,
		KubedoopConfigDir, kafkav1alpha1.ServerFileName,
		listenerConfig.ListenersString(),
		listenerConfig.AdvertisedListenersString(),
		listenerConfig.ListenerSecurityProtocolMapString(),
	)

	args := []string{
		// The framework mounts the config ConfigMap read-only at KubedoopConfigDirMount;
		// copy it into the writable config dir (server.properties, security.properties and
		// log4j.properties included — all in that ConfigMap).
		// The glob strips any trailing slash from the mount constant first, so the copy
		// works regardless of how the framework formats the path; quoting guards the
		// non-glob expansions.
		fmt.Sprintf(`CONFIG_DIR_MOUNT=%s
CONFIG_DIR=%s
mkdir --parents "${CONFIG_DIR}"
echo copying "${CONFIG_DIR_MOUNT}" to "${CONFIG_DIR}"
cp -RL "${CONFIG_DIR_MOUNT%%/}"/* "${CONFIG_DIR}"`, opgoconstant.KubedoopConfigDirMount, KubedoopConfigDir),
	}

	if kafkaSecurity.IsKerberosEnabled() {
		kerberosDir := secretProvisioner.MustPath(kafkav1alpha1.KerberosVolumeName)
		args = append(args,
			fmt.Sprintf(`export KERBEROS_REALM=$(grep -oP 'default_realm = \K.*' %s/krb5.conf)`, kerberosDir))

		keytab := fmt.Sprintf("%s/keytab", kerberosDir)
		serviceName := kafkav1alpha1.KerberosServiceName
		brokerAddress := NodeAddressCmd(listenerProvisioner.MustPath(kafkav1alpha1.ListenerBrokerVolumeName))
		bootstrapAddress := NodeAddressCmd(listenerProvisioner.MustPath(kafkav1alpha1.ListenerBootstrapVolumeName))
		launch += fmt.Sprintf(
			` --override "listener.name.client.gssapi.sasl.jaas.config=com.sun.security.auth.module.Krb5LoginModule required useKeyTab=true storeKey=true isInitiator=false keyTab=\"%s\" principal=\"%s/%s@$KERBEROS_REALM\";" --override "listener.name.bootstrap.gssapi.sasl.jaas.config=com.sun.security.auth.module.Krb5LoginModule required useKeyTab=true storeKey=true isInitiator=false keyTab=\"%s\" principal=\"%s/%s@$KERBEROS_REALM\";"`,
			keytab, serviceName, brokerAddress,
			keytab, serviceName, bootstrapAddress,
		)
	}

	args = append(args, `echo "Starting Kafka"`, launch)
	return []string{strings.Join(args, "\n")}, nil
}

// getEnvVars returns environment variables for the main container.
func (h *KafkaRoleGroupHandler) getEnvVars(
	cr *kafkav1alpha1.KafkaCluster,
	kafkaSecurity *security.KafkaSecurity,
	secretProvisioner *opgosecurity.SecretProvisioner,
) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name: EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: EnvNode,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: EnvZookeeperConnections,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cr.Spec.ClusterConfig.ZookeeperConfigMapName,
					},
					Key: ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  EnvKafkaLog4jOpts,
			Value: fmt.Sprintf("-Dlog4j.configuration=file:%s/%s", KubedoopConfigDir, kafkav1alpha1.Log4jFileName),
		},
		{
			Name: EnvJvmArgs,
			Value: fmt.Sprintf("-Djava.security.properties=%s/%s -javaagent:%s/jmx/jmx_prometheus_javaagent.jar=%d:%s/jmx/config.yaml",
				KubedoopConfigDir, kafkav1alpha1.SecurityFileName, KubedoopRoot, kafkav1alpha1.MetricsPort, KubedoopRoot),
		},
	}

	if kafkaSecurity.IsKerberosEnabled() {
		krb5Conf := fmt.Sprintf("%s/krb5.conf", secretProvisioner.MustPath(kafkav1alpha1.KerberosVolumeName))
		envs = append(envs,
			corev1.EnvVar{Name: "KRB5_CONFIG", Value: krb5Conf},
			corev1.EnvVar{Name: "KAFKA_OPTS", Value: fmt.Sprintf("-Djava.security.krb5.conf=%s", krb5Conf)},
		)
	}

	// KAFKA_HEAP_OPTS is derived from the effective memory limit in ResolveRoleGroup and
	// arrives through the merged env (beneath the user's envOverrides).

	return envs
}

// kafkaLivenessProbe returns the liveness probe (TCP on the client port).
func kafkaLivenessProbe(kafkaSecurity *security.KafkaSecurity) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    6,
		InitialDelaySeconds: 20,
		PeriodSeconds:       30,
		SuccessThreshold:    1,
		TimeoutSeconds:      5,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkaSecurity.ClientPortName())},
		},
	}
}

// kafkaReadinessProbe returns the readiness probe (TCP on the client port).
func kafkaReadinessProbe(kafkaSecurity *security.KafkaSecurity) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold:    3,
		InitialDelaySeconds: 20,
		PeriodSeconds:       30,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(kafkaSecurity.ClientPortName())},
		},
	}
}
