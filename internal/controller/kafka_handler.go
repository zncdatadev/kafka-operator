package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/listener"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

var logger = ctrl.Log.WithName("kafka-handler")

// parseSecretLifetime parses a certificate lifetime expression. On top of Go duration
// syntax it supports the day suffix documented in the CRD ("1d", "7d", "30d") —
// secret-operator itself parses the annotation with Go duration syntax only.
func parseSecretLifetime(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day expression %q: %w", s, err)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
}

// RBAC for the GenericReconciler-driven KafkaCluster controller — the canonical set from
// operator-go docs/security.md §3.3, plus the bootstrap Listener CRs kafka ships as
// ExtraResources. Deliberate absences: no `delete` on serviceaccounts (reclaimed by
// owner-reference GC) and no `update`/`patch` on the CR body (the framework writes only
// Status().Update; add them back only if a finalizer is ever registered).
//
// +kubebuilder:rbac:groups=kafka.kubedoop.dev,resources=kafkaclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=kafka.kubedoop.dev,resources=kafkaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kafka.kubedoop.dev,resources=kafkaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=listeners.kubedoop.dev,resources=listeners,verbs=get;list;watch;create;update;patch;delete

// LabelDomain is the product domain used for identity (selector) labels:
// kafka.kubedoop.dev/{cluster,role,role-group}. The product-domain prefix guarantees
// these selectors never match another product's pods.
const LabelDomain = "kafka.kubedoop.dev"

// LabelListenerBootstrap marks the per-role-group bootstrap Listener so discovery can
// find all bootstrap listeners of a cluster with a label selector.
const (
	LabelListenerBootstrap = "app.kubernetes.io/listener-bootstrap"
	LabelValueTrue         = "true"
)

// kafkaServerLogging is the single source of truth for the Kafka main container's logging.
// It drives both BaseRoleGroupHandler.LoggingContainers (the framework's shared Vector log
// volume producer/consumer wiring) and the log4j.properties + vector.yaml rendered into the
// role group ConfigMap. Kafka 3.x logs via reload4j, so the log4j 1.x generator is used.
// The framework derives the rolling file path from the container name and framework
// ("/kubedoop/log/kafka/kafka.log4j.xml", XMLLayout), which the Vector sidecar edge-parses.
var kafkaServerLogging = productlogging.ContainerLogging{
	Container: kafkav1alpha1.KafkaContainerName,
	Framework: productlogging.LoggingFrameworkLog4j,
	Pattern:   "[%d] %p %m (%c)%n",
}

// KafkaRoleGroupHandler builds the resources for a Kafka broker role group.
//
// It embeds reconciler.BaseRoleGroupHandler to inherit the framework's canonical resource
// construction (labels, headless/client Services, builder-built StatefulSet with the data
// PVC, PodDisruptionBudget, sidecar injection, CSI volume injection via VolumeProviders),
// then customizes the returned StatefulSet and ConfigMap with Kafka specifics (start
// command with listener overrides, TCP probes, server.properties). The per-role-group
// bootstrap Listener CR is shipped through RoleGroupResources.ExtraResources so the
// framework applies it before the StatefulSet (pods mount a CSI volume referencing it).
type KafkaRoleGroupHandler struct {
	*reconciler.BaseRoleGroupHandler[*kafkav1alpha1.KafkaCluster]
}

var _ reconciler.RoleGroupHandler[*kafkav1alpha1.KafkaCluster] = &KafkaRoleGroupHandler{}
var _ reconciler.RoleProvider[*kafkav1alpha1.KafkaCluster] = &KafkaRoleGroupHandler{}

// NewKafkaRoleGroupHandler creates the handler. It carries only reconcile-invariant
// collaborators: everything a ROLE is made of is declared per reconcile by DeclareRoles,
// with the cr in hand.
func NewKafkaRoleGroupHandler(scheme *runtime.Scheme) *KafkaRoleGroupHandler {
	base := reconciler.NewBaseRoleGroupHandler[*kafkav1alpha1.KafkaCluster](scheme)
	// Product-owned identity labels drive all resource selectors (decoupled from the
	// descriptive app.kubernetes.io/* labels).
	base.LabelDomain = LabelDomain
	return &KafkaRoleGroupHandler{BaseRoleGroupHandler: base}
}

// DeclareRoles implements reconciler.RoleProvider: the broker role's facts, produced once
// per reconcile pass with the cr in hand — a port that moves because the CR enabled TLS is
// computed here, from THIS cr, never from process-wide handler state.
func (h *KafkaRoleGroupHandler) DeclareRoles(
	_ context.Context, _ ctrlclient.Client, cr *kafkav1alpha1.KafkaCluster,
) (reconciler.RoleCatalog, error) {
	kafkaSecurity := security.NewKafkaSecurity(cr)

	// Kafka's default scheduling posture: prefer spreading brokers of the same cluster
	// across nodes. Declared as a config default, so anything the user states in
	// config.affinity wins member-by-member.
	affinity, err := reconciler.EncodeAffinity(&corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				reconciler.PreferredAffinityTerm(70, corev1.LabelHostname,
					reconciler.RoleSelectorLabels(cr.Name, kafkav1alpha1.BrokerRoleName)),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode default affinity: %w", err)
	}

	return reconciler.RoleCatalog{
		kafkav1alpha1.BrokerRoleName: {
			// The primary container is named "kafka"; the log producer declaration below
			// names the same container, keeping config and shared log volume in lockstep.
			MainContainerName: kafkav1alpha1.KafkaContainerName,
			// Ports[0] is the client port — the one that means "this broker can serve".
			ContainerPorts: KafkaContainerPorts(kafkaSecurity),
			ServicePorts:   kafkaServicePorts(kafkaSecurity),
			// The args (start script with listener overrides) are per role group and are set
			// in BuildResources; only the interpreter is a role-wide fact.
			Command:        []string{"/bin/bash", "-x", "-euo", "pipefail", "-c"},
			ReadinessProbe: kafkaReadinessProbe(kafkaSecurity),
			LivenessProbe:  kafkaLivenessProbe(kafkaSecurity),
			// Topic data must be persistent.
			DataVolume: &reconciler.DataVolume{MountPath: KubedoopDataDir},
			// Brokers must resolve each other's DNS before readiness.
			PublishNotReadyAddresses: true,
			LogProducers:             []productlogging.ContainerLogging{kafkaServerLogging},
			// Kafka's defaults for the framework-owned config half, folded BENEATH the
			// user's role/role-group config: resource guarantees (the memory limit also
			// drives KAFKA_HEAP_OPTS), pre-framework termination grace, and the
			// anti-affinity above.
			ConfigDefaults: &commonsv1alpha1.RoleGroupConfigSpec{
				Affinity:                affinity,
				GracefulShutdownTimeout: ptr.To(defaultGracefulShutdownTimeout.String()),
				Resources: &commonsv1alpha1.ResourcesSpec{
					CPU: &commonsv1alpha1.CPUResource{
						Min: ptr.To(resource.MustParse(defaultCPURequest)),
						Max: ptr.To(resource.MustParse(defaultCPULimit)),
					},
					Memory: &commonsv1alpha1.MemoryResource{
						Limit: ptr.To(resource.MustParse(defaultMemoryLimit)),
					},
					Storage: &commonsv1alpha1.StorageResource{
						Capacity: ptr.To(resource.MustParse(defaultStorageCapacity)),
					},
				},
			},
		},
	}, nil
}

// BuildResources builds all Kubernetes resources for a Kafka broker role group.
func (h *KafkaRoleGroupHandler) BuildResources(
	ctx context.Context,
	k8sClient ctrlclient.Client,
	cr *kafkav1alpha1.KafkaCluster,
	buildCtx *reconciler.RoleGroupBuildContext,
) (*reconciler.RoleGroupResources, error) {
	if buildCtx.RoleName != kafkav1alpha1.BrokerRoleName {
		return nil, fmt.Errorf("unsupported role: %s", buildCtx.RoleName)
	}
	if cr.Spec.ClusterConfig == nil || cr.Spec.ClusterConfig.ZookeeperConfigMapName == "" {
		return nil, fmt.Errorf("spec.clusterConfig.zookeeperConfigMapName is required")
	}

	// Resolve Kafka-specific inputs.
	kafkaSecurity := security.NewKafkaSecurity(cr)
	brokerCfg := resolveBrokerConfig(cr, buildCtx.RoleGroupName)
	secretProvisioner := h.buildSecretProvisioner(kafkaSecurity, brokerCfg)
	bootstrapListenerName := BootstrapListenerName(buildCtx.ResourceName)
	listenerProvisioner := h.buildListenerProvisioner(brokerCfg, bootstrapListenerName)

	// Ports, probes, command, data volume and the Kafka config defaults are declared in
	// DeclareRoles (per reconcile pass, from this cr); resource defaults arrive via the
	// declaration's ConfigDefaults through the framework fold.

	// Hand the CSI volumes (TLS keystores, Kerberos keytab, listener addresses) to the
	// framework so base.BuildResources() injects them into the pod and the main container.
	// VolumeProviders lives on the build context (rebuilt each reconcile), so registrations
	// never accumulate across reconciles or leak across CRs.
	buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, secretProvisioner, listenerProvisioner)

	// Let the framework build the skeleton: canonical labels, headless Service (with
	// PublishNotReadyAddresses), client Service, StatefulSet (data PVC + injected
	// sidecars + CSI volumes), and PodDisruptionBudget.
	res, err := h.BaseRoleGroupHandler.BuildResources(ctx, k8sClient, cr, buildCtx)
	if err != nil {
		return nil, fmt.Errorf("base build failed: %w", err)
	}

	// Customize the StatefulSet with Kafka specifics.
	if err := h.customizeStatefulSet(res.StatefulSet, buildCtx, cr, kafkaSecurity, secretProvisioner, listenerProvisioner); err != nil {
		return nil, err
	}

	// Replace the ConfigMap with computed Kafka config (server.properties,
	// security.properties, log4j.properties, vector.yaml). Reuse the framework labels base
	// put on the StatefulSet.
	cm, err := h.buildConfigMap(buildCtx, res.StatefulSet.Labels, kafkaSecurity, secretProvisioner)
	if err != nil {
		return nil, fmt.Errorf("failed to build configmap: %w", err)
	}
	res.ConfigMap = cm

	// Metrics Service (headless with Prometheus scrape annotations). Its selector uses the
	// identity labels, consistent with the other role-group resources.
	res.MetricsService = builder.NewMetricsServiceBuilder(
		buildCtx.ResourceName,
		buildCtx.ClusterNamespace,
		kafkav1alpha1.MetricsPort,
		res.StatefulSet.Labels,
	).WithSelector(h.SelectorLabels(buildCtx)).
		// Target the container port by name so renumbering never breaks the Service
		// (pre-framework parity).
		WithTargetPortName(kafkav1alpha1.MetricsPortName).Build()

	// The per-role-group bootstrap Listener gives clients a stable bootstrap address. It is
	// shipped as an extra resource so the framework applies it BEFORE the StatefulSet: the
	// pods' listener-bootstrap CSI volume references it by name, and would otherwise hang in
	// ContainerCreating.
	res.ExtraResources = append(res.ExtraResources,
		h.buildBootstrapListener(bootstrapListenerName, buildCtx, res.StatefulSet.Labels, brokerCfg, kafkaSecurity))

	return res, nil
}

// buildSecretProvisioner declares all CSI secret volumes needed by the broker based on the
// security configuration. TLS keystore volumes are scoped to the listener volumes (plus
// pod and node) so certificates carry the listener addresses as SANs.
func (h *KafkaRoleGroupHandler) buildSecretProvisioner(
	kafkaSecurity *security.KafkaSecurity,
	brokerCfg *brokerConfig,
) *opgosecurity.SecretProvisioner {
	provisioner := opgosecurity.NewSecretProvisioner()

	listenerScopes := strings.Join([]string{
		string(opgosecurity.ListenerVolumeScope) + "=" + kafkav1alpha1.ListenerBrokerVolumeName,
		string(opgosecurity.ListenerVolumeScope) + "=" + kafkav1alpha1.ListenerBootstrapVolumeName,
	}, opgosecurity.CommonDelimiter)

	tlsScope := strings.Join([]string{
		listenerScopes,
		string(opgosecurity.PodScope),
		string(opgosecurity.NodeScope),
	}, opgosecurity.CommonDelimiter)

	registerTLS := func(volumeName, secretClass string) {
		reg := opgosecurity.TLS(volumeName, secretClass).WithScope(tlsScope)
		if kafkaSecurity.SSLStorePassword != "" {
			reg.WithPassword(kafkaSecurity.SSLStorePassword)
		}
		if brokerCfg.RequestedSecretLifeTime != "" {
			// The CRD documents day expressions ("7d", "30d"); secret-operator parses the
			// annotation with Go duration syntax, so convert before handing it to the
			// framework. Invalid expressions are skipped (secret-operator would reject the
			// whole volume otherwise, wedging the pod in Pending).
			if lifetime, err := parseSecretLifetime(brokerCfg.RequestedSecretLifeTime); err == nil {
				reg.WithCertLifetime(lifetime)
			} else {
				logger.V(0).Info("ignoring invalid requestedSecretLifeTime",
					"value", brokerCfg.RequestedSecretLifeTime, "error", err.Error())
			}
		}
		provisioner.Register(reg)
	}

	if serverClass := kafkaSecurity.TlsServerSecretClass(); serverClass != "" {
		registerTLS(kafkav1alpha1.TLSKeystoreServerVolumeName, serverClass)
	}

	if internalClass := kafkaSecurity.TlsInternalSecretClass(); internalClass != "" {
		registerTLS(kafkav1alpha1.TLSKeystoreInternalVolumeName, internalClass)
	}

	if kafkaSecurity.IsKerberosEnabled() {
		provisioner.Register(opgosecurity.KerberosVolume(
			kafkav1alpha1.KerberosVolumeName,
			kafkaSecurity.KerberosSecretClass(),
			kafkav1alpha1.KerberosServiceName,
		).WithScope(listenerScopes))
	}

	return provisioner
}

// buildListenerProvisioner declares the listener CSI volumes: the per-broker listener
// (provisioned from the role group's broker listener class) and the bootstrap listener
// (referencing the pre-created bootstrap Listener by name).
func (h *KafkaRoleGroupHandler) buildListenerProvisioner(
	brokerCfg *brokerConfig,
	bootstrapListenerName string,
) *listener.ListenerProvisioner {
	return listener.NewProvisioner().RegisterVolume(
		listener.NewVolume(kafkav1alpha1.ListenerBrokerVolumeName, listener.ListenerClass(brokerCfg.BrokerListenerClass)),
		// The bootstrap volume references the pre-created bootstrap Listener by name; the
		// class lives on the Listener CR itself (the registration omits the class
		// annotation entirely for by-name references).
		listener.NewVolume(kafkav1alpha1.ListenerBootstrapVolumeName, "").
			WithListenerName(bootstrapListenerName),
	)
}

// buildBootstrapListener builds the per-role-group bootstrap Listener CR.
func (h *KafkaRoleGroupHandler) buildBootstrapListener(
	name string,
	buildCtx *reconciler.RoleGroupBuildContext,
	labels map[string]string,
	brokerCfg *brokerConfig,
	kafkaSecurity *security.KafkaSecurity,
) ctrlclient.Object {
	return NewBootstrapListener(name, buildCtx.ClusterNamespace, labels, brokerCfg.BootstrapListenerClass, kafkaSecurity)
}

// brokerConfig carries the Kafka-specific role group settings that live outside the
// framework's generic RoleGroupConfigSpec (and therefore outside MergedConfig): the
// listener classes and the requested certificate lifetime. Resolution order is
// role group config > role config > default.
type brokerConfig struct {
	BrokerListenerClass     string
	BootstrapListenerClass  string
	RequestedSecretLifeTime string
}

const (
	defaultListenerClass           = "cluster-internal"
	defaultRequestedSecretLifeTime = "1d"
)

// resolveBrokerConfig resolves the Kafka-specific broker settings for a role group.
func resolveBrokerConfig(cr *kafkav1alpha1.KafkaCluster, roleGroupName string) *brokerConfig {
	cfg := &brokerConfig{
		BrokerListenerClass:     defaultListenerClass,
		BootstrapListenerClass:  defaultListenerClass,
		RequestedSecretLifeTime: defaultRequestedSecretLifeTime,
	}

	apply := func(spec *kafkav1alpha1.BrokersConfigSpec) {
		if spec == nil {
			return
		}
		if spec.BrokerListenerClass != "" {
			cfg.BrokerListenerClass = spec.BrokerListenerClass
		}
		if spec.BootstrapListenerClass != "" {
			cfg.BootstrapListenerClass = spec.BootstrapListenerClass
		}
		if spec.RequestedSecretLifeTime != "" {
			cfg.RequestedSecretLifeTime = spec.RequestedSecretLifeTime
		}
	}

	if cr.Spec.Brokers == nil {
		return cfg
	}
	// Role-level config first, then the role group's own config on top.
	apply(cr.Spec.Brokers.Config)
	if rg, ok := cr.Spec.Brokers.RoleGroups[roleGroupName]; ok && rg != nil {
		apply(rg.Config)
	}
	return cfg
}
