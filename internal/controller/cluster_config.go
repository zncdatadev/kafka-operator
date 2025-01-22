package controller

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/constants"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

const (
	RoleName = "broker"
)

var clusterConfigLogger = ctrl.Log.WithName("clusterconfig")

type OverrideConfiguration interface {
	ComputeEnv() (map[string]string, error)
	ComputeFile() (map[string]map[string]string, error)
	ComputeCli() ([]string, error)
}

var _ OverrideConfiguration = &KafkaConfig{}

type KafkaConfig struct {
	commonsv1alpha1.RoleGroupConfigSpec

	BootstrapListenerClass string

	BrokerListenerClass string

	RequestedSecretLifetime string
}

// ComputeCli implements OverrideConfiguration.
func (k *KafkaConfig) ComputeCli() ([]string, error) {
	return nil, fmt.Errorf("unimplemented")
}

// ComputeEnv implements OverrideConfiguration.
func (k *KafkaConfig) ComputeEnv() (map[string]string, error) {
	return nil, fmt.Errorf("unimplemented")
}

// ComputeFile implements OverrideConfiguration.
func (k *KafkaConfig) ComputeFile() (map[string]map[string]string, error) {
	return map[string]map[string]string{
		ServerPropertiesFilename: {
			"zookeeper.connection.timeout.ms": "18000",
			"controlled.shutdown.enable":      "true",
			"log.dirs":                        path.Join(constants.KubedoopDataDir, "topicdata"),
		},
		SecurityPropertiesFilename: {
			"networkaddress.cache.ttl":          "30",
			"networkaddress.cache.negative.ttl": "0",
		},
	}, nil
}

func DefaultKafkaConfig(clusterName string) KafkaConfig {

	rawAffinity, err := json.Marshal(defaultAffinity(RoleName, clusterName))
	if err != nil {
		clusterConfigLogger.Error(err, "Failed to marshal affinity")
	}

	return KafkaConfig{
		RoleGroupConfigSpec: commonsv1alpha1.RoleGroupConfigSpec{
			Affinity: &runtime.RawExtension{
				Raw: rawAffinity,
			},
			GracefulShutdownTimeout: "30s",
			Logging: &commonsv1alpha1.LoggingSpec{
				EnableVectorAgent: ptr.To(false),
				Containers:        nil,
			},
			Resources: &commonsv1alpha1.ResourcesSpec{
				CPU: &commonsv1alpha1.CPUResource{
					Max: resource.MustParse("1000m"),
					Min: resource.MustParse("250m"),
				},
				Memory: &commonsv1alpha1.MemoryResource{
					Limit: resource.MustParse("1Gi"),
				},
				Storage: &commonsv1alpha1.StorageResource{
					Capacity: resource.MustParse("2Gi"),
				},
			},
		},
		BootstrapListenerClass:  "cluster-internal",
		BrokerListenerClass:     "cluster-internal",
		RequestedSecretLifetime: "1d",
	}
}

func DefaultLogging() {
	// todo
}

func defaultAffinity(role string, crName string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 70,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								constants.LabelKubernetesInstance:  crName,
								constants.LabelKubernetesComponent: role,
							},
						},
						TopologyKey: corev1.LabelHostname,
					},
				},
			},
		},
	}
}

// mergeOverrides merges default configurations into user overrides
func mergeOverrides(userOverrides *commonsv1alpha1.OverridesSpec, defaultConfig KafkaConfig) error {
	if userOverrides == nil {
		userOverrides = &commonsv1alpha1.OverridesSpec{}
	}

	// Initialize all required maps
	if userOverrides.CliOverrides == nil {
		userOverrides.CliOverrides = make([]string, 0)
	}
	if userOverrides.EnvOverrides == nil {
		userOverrides.EnvOverrides = make(map[string]string)
	}
	if userOverrides.ConfigOverrides == nil {
		userOverrides.ConfigOverrides = make(map[string]map[string]string)
	}

	// Merge CLI configurations
	if cliConfig, err := defaultConfig.ComputeCli(); err == nil {
		for _, cmd := range cliConfig {
			if !slices.Contains(userOverrides.CliOverrides, cmd) {
				userOverrides.CliOverrides = append(userOverrides.CliOverrides, cmd)
			}
		}
	}

	// Merge environment variable configurations
	if envConfig, err := defaultConfig.ComputeEnv(); err == nil {
		for k, v := range envConfig {
			if _, exists := userOverrides.EnvOverrides[k]; !exists {
				userOverrides.EnvOverrides[k] = v
			}
		}
	}

	// Merge file configurations
	if fileConfig, err := defaultConfig.ComputeFile(); err == nil {
		for filename, configs := range fileConfig {
			if _, exists := userOverrides.ConfigOverrides[filename]; !exists {
				userOverrides.ConfigOverrides[filename] = configs
			} else {
				// Merge configurations within the same file
				for k, v := range configs {
					if _, exists := userOverrides.ConfigOverrides[filename][k]; !exists {
						userOverrides.ConfigOverrides[filename][k] = v
					}
				}
			}
		}
	}

	return nil
}

func MergeFromUserConfig(
	userConfig *kafkav1alpha1.BrokersConfigSpec,
	userOverrides *commonsv1alpha1.OverridesSpec,
	clusterName string,
) error {
	if userConfig == nil {
		return fmt.Errorf("userConfig cannot be nil")
	}

	defaultConfig := DefaultKafkaConfig(clusterName)

	// Merge base configurations
	if err := mergeBaseConfig(userConfig, defaultConfig); err != nil {
		return err
	}

	// Merge override configurations
	return mergeOverrides(userOverrides, defaultConfig)
}

// mergeBaseConfig merges base configuration items
func mergeBaseConfig(userConfig *kafkav1alpha1.BrokersConfigSpec, defaultConfig KafkaConfig) error {
	// Affinity
	if userConfig.Affinity == nil {
		userConfig.Affinity = defaultConfig.Affinity
	}

	// GracefulShutdownTimeout
	if userConfig.GracefulShutdownTimeout == "" {
		userConfig.GracefulShutdownTimeout = defaultConfig.GracefulShutdownTimeout
	}

	// Logging
	if userConfig.Logging == nil {
		userConfig.Logging = defaultConfig.Logging
	}

	// Resources
	if userConfig.Resources == nil {
		userConfig.Resources = defaultConfig.Resources
	} else {
		mergeResources(userConfig.Resources, defaultConfig.Resources)
	}

	// Kafka specific fields
	if userConfig.BootstrapListenerClass == "" {
		userConfig.BootstrapListenerClass = defaultConfig.BootstrapListenerClass
	}
	if userConfig.BrokerListenerClass == "" {
		userConfig.BrokerListenerClass = defaultConfig.BrokerListenerClass
	}
	if userConfig.RequestedSecretLifeTime == "" {
		userConfig.RequestedSecretLifeTime = defaultConfig.RequestedSecretLifetime
	}

	return nil
}

// mergeResources merges resource configurations
func mergeResources(userResources, defaultResources *commonsv1alpha1.ResourcesSpec) {
	if userResources.CPU == nil {
		userResources.CPU = defaultResources.CPU
	}
	if userResources.Memory == nil {
		userResources.Memory = defaultResources.Memory
	}
	if userResources.Storage == nil {
		userResources.Storage = defaultResources.Storage
	}
}
