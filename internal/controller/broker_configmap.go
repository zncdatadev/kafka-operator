package controller

import (
	"context"
	"fmt"
	"maps"
	"path"
	"sort"
	"strings"

	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
	"github.com/zncdatadev/kafka-operator/internal/util"
)

// ResolveRoleGroup is the framework RoleGroupResolver hook: what Kafka DERIVES from a role
// group's effective config — the default config-file content and the JVM heap sized from
// the effective memory limit. Both are folded BENEATH the user's own configOverrides and
// envOverrides per key, so every derived value stays a default the user can refine.
func ResolveRoleGroup(
	_ context.Context, _ client.Client, _ *kafkav1alpha1.KafkaCluster,
	buildCtx *reconciler.RoleGroupBuildContext,
) (*reconciler.Contribution, error) {
	contribution := &reconciler.Contribution{
		ConfigOverrides: map[string]map[string]string{
			kafkav1alpha1.ServerFileName: {
				"zookeeper.connection.timeout.ms": "18000",
				"controlled.shutdown.enable":      "true",
				"log.dirs":                        path.Join(KubedoopDataDir, "topicdata"),
			},
			kafkav1alpha1.SecurityFileName: {
				"networkaddress.cache.ttl":          "30",
				"networkaddress.cache.negative.ttl": "0",
			},
		},
	}

	// Heap limit from the EFFECTIVE memory limit (80%): the fold has already resolved the
	// declaration's default against the user's role/role-group config, so a user raising
	// the memory limit raises the heap with it — and an explicit KAFKA_HEAP_OPTS in
	// envOverrides still wins.
	if resources := buildCtx.EffectiveConfig().Resources; resources != nil &&
		resources.Memory != nil && resources.Memory.Limit != nil {
		if heap := int(util.QuantityToMB(*resources.Memory.Limit) * 0.8); heap > 0 {
			contribution.EnvVars = map[string]string{
				EnvKafkaHeapOpts: fmt.Sprintf("-Xmx%dm", heap),
			}
		}
	}

	return contribution, nil
}

// buildConfigMap creates the ConfigMap for a broker role group: server.properties (merged
// defaults + overrides + TLS/Kerberos settings), security.properties, and the
// framework-owned logging entries (log4j.properties and, when Vector is enabled,
// vector.yaml).
func (h *KafkaRoleGroupHandler) buildConfigMap(
	buildCtx *reconciler.RoleGroupBuildContext,
	labels map[string]string,
	kafkaSecurity *security.KafkaSecurity,
	secretProvisioner *opgosecurity.SecretProvisioner,
) (*corev1.ConfigMap, error) {
	data := make(map[string]string)

	data[kafkav1alpha1.ServerFileName] = h.generateServerProperties(buildCtx, kafkaSecurity, secretProvisioner)
	data[kafkav1alpha1.SecurityFileName] = h.generateSecurityProperties(buildCtx)

	// Framework-owned logging config: log4j.properties (from the deep-merged CRD logging
	// spec, with the file appender gated on Vector) and, when Vector is enabled and the CR
	// exposes the aggregator ConfigMap, vector.yaml. The declaration's LogProducers is the
	// single statement that also drives the shared log volume, so config and volume stay
	// in lockstep.
	loggingData, err := reconciler.RenderLoggingConfigMapData(buildCtx, buildCtx.Declaration.LogProducers)
	if err != nil {
		return nil, fmt.Errorf("failed to render logging config: %w", err)
	}
	maps.Copy(data, loggingData)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildCtx.ResourceName,
			Namespace: buildCtx.ClusterNamespace,
			Labels:    labels,
		},
		Data: data,
	}, nil
}

// generateServerProperties renders server.properties: the merged config file (product
// defaults + role/role-group overrides, already folded by the framework) plus the
// TLS/Kerberos settings, which are authoritative and therefore applied last.
func (h *KafkaRoleGroupHandler) generateServerProperties(
	buildCtx *reconciler.RoleGroupBuildContext,
	kafkaSecurity *security.KafkaSecurity,
	secretProvisioner *opgosecurity.SecretProvisioner,
) string {
	properties := make(map[string]string)
	if buildCtx.MergedConfig != nil {
		if merged, ok := buildCtx.MergedConfig.ConfigFiles[kafkav1alpha1.ServerFileName]; ok {
			maps.Copy(properties, merged)
		}
	}
	maps.Copy(properties, kafkaSecurity.ConfigSettings(secretProvisioner))
	return toProperties(properties)
}

// generateSecurityProperties renders security.properties (JVM security settings) from the
// merged config file.
func (h *KafkaRoleGroupHandler) generateSecurityProperties(buildCtx *reconciler.RoleGroupBuildContext) string {
	if buildCtx.MergedConfig != nil {
		if merged, ok := buildCtx.MergedConfig.ConfigFiles[kafkav1alpha1.SecurityFileName]; ok {
			return toProperties(merged)
		}
	}
	return ""
}

// toProperties renders a map as a Java properties file with sorted keys, so the output is
// deterministic across reconciles (no spurious ConfigMap updates).
func toProperties(config map[string]string) string {
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(config[k])
		b.WriteString("\n")
	}
	return b.String()
}
