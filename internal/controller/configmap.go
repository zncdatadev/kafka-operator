package controller

import (
	"context"
	"errors"
	"maps"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/config/properties"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

const (
	ServerPropertiesFilename   = "server.properties"
	SecurityPropertiesFilename = "security.properties"
	Log4jPropertiesFilename    = "log4j.properties"
	VectorConfigFilename       = "vector.yaml"

	KafkaLog4jFilename = "kafka.log4j.xml"

	ConsoleConversionPattern = "[%d] %p %m (%c)%n"
)

func NewKafkaConfigmapReconciler(
	ctx context.Context,
	client *client.Client,
	clusterConfig *kafkav1alpha1.ClusterConfigSpec,
	kafkaTlsSecurity *security.KafkaTlsSecurity,
	roleGroupInf *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) reconciler.ResourceReconciler[builder.ConfigBuilder] {
	builder := NewKafkaConfigmapBuilder(
		client,
		roleGroupInf,
		clusterConfig,
		kafkaTlsSecurity,
		overrides,
		roleGroupConfig,
	)
	return reconciler.NewGenericResourceReconciler(client, builder)
}

func NewKafkaConfigmapBuilder(
	client *client.Client,
	roleGroupInfo *reconciler.RoleGroupInfo,
	clusterConfig *kafkav1alpha1.ClusterConfigSpec,
	kafkaTlsSecurity *security.KafkaTlsSecurity,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) builder.ConfigBuilder {
	return &KafkaConfigmapBuilder{
		ConfigMapBuilder: *builder.NewConfigMapBuilder(
			client,
			roleGroupInfo.GetFullName(),
			func(o *builder.Options) {
				o.Labels = roleGroupInfo.GetLabels()
				o.Annotations = roleGroupInfo.GetAnnotations()
			},
		),
		ClusterConfig:   clusterConfig,
		kafkaSecurity:   kafkaTlsSecurity,
		overrides:       overrides,
		roleGroupConfig: roleGroupConfig,
		ClusterName:     roleGroupInfo.ClusterName,
		RoleName:        roleGroupInfo.RoleName,
		RoleGroupName:   roleGroupInfo.RoleGroupName,
	}
}

type KafkaConfigmapBuilder struct {
	builder.ConfigMapBuilder

	ClusterConfig *kafkav1alpha1.ClusterConfigSpec

	kafkaSecurity   *security.KafkaTlsSecurity
	overrides       *commonsv1alpha1.OverridesSpec
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec

	ClusterName   string
	RoleName      string
	RoleGroupName string
}

func (b *KafkaConfigmapBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	propertyFiles := map[string]func() (string, error){
		ServerPropertiesFilename:   b.buildServerProperties,   // server.properties
		SecurityPropertiesFilename: b.buildSecurityProperties, // security.properties
		Log4jPropertiesFilename:    b.buildLog4jProperties,    // log4j.properties
	}

	for filename, builder := range propertyFiles {
		content, err := builder()
		if err != nil {
			return nil, err
		}
		if content != "" {
			b.AddItem(filename, content)
		}
	}

	// vector config
	if IsVectorEnable(b.roleGroupConfig.Logging) {
		if vectorConfig, err := b.buildVectorConfig(ctx); err != nil {
			return nil, err
		} else if vectorConfig != "" {
			b.AddItem(VectorConfigFilename, vectorConfig) // vector.yaml
		}
	}

	return b.GetObject(), nil
}

// secruity properties
func (b *KafkaConfigmapBuilder) buildSecurityProperties() (string, error) {
	if b.overrides != nil && b.overrides.ConfigOverrides != nil {
		if data, ok := b.overrides.ConfigOverrides[SecurityPropertiesFilename]; ok {
			propertyLoader := properties.NewPropertiesFromMap(data)
			return propertyLoader.Marshal()
		}
	}
	return "", nil
}

// server properties
func (b *KafkaConfigmapBuilder) buildServerProperties() (string, error) {
	data := make(map[string]string)
	if b.overrides != nil && b.overrides.ConfigOverrides != nil {
		data, _ = b.overrides.ConfigOverrides[ServerPropertiesFilename]
	}

	maps.Copy(data, b.kafkaSecurity.ConfigSettings()) // tls

	propertyLoader := properties.NewPropertiesFromMap(data)
	return propertyLoader.Marshal()
}

// log4j properties
func (b *KafkaConfigmapBuilder) buildLog4jProperties() (string, error) {

	var loggingSpec *commonsv1alpha1.LoggingConfigSpec
	if b.roleGroupConfig != nil && b.roleGroupConfig.Logging != nil && b.roleGroupConfig.Logging.Containers != nil {
		if mainContainerLogging, ok := b.roleGroupConfig.Logging.Containers[string(Kafka)]; ok {
			loggingSpec = &mainContainerLogging
		}
	}
	loggingConfig, err := productlogging.NewConfigGenerator(
		loggingSpec,
		string(Kafka),
		KafkaLog4jFilename,
		productlogging.LogTypeLog4j,
		func(cgo *productlogging.ConfigGeneratorOption) {
			cgo.ConsoleHandlerFormatter = ptr.To(ConsoleConversionPattern)
		},
	)
	if err != nil {
		return "", err
	}

	return loggingConfig.Content()
}

// vector config
func (b *KafkaConfigmapBuilder) buildVectorConfig(ctx context.Context) (string, error) {
	if b.roleGroupConfig != nil && b.roleGroupConfig.Logging != nil && b.roleGroupConfig.Logging.EnableVectorAgent != nil {
		if b.ClusterConfig.VectorAggregatorConfigMapName == "" {
			return "", errors.New("vector is enabled but vectorAggregatorConfigMapName is not set")
		}
		if *b.roleGroupConfig.Logging.EnableVectorAgent {
			s, err := productlogging.MakeVectorYaml(
				ctx,
				b.Client.Client,
				b.Client.GetOwnerNamespace(),
				b.ClusterName,
				b.RoleName,
				b.RoleGroupName,
				b.ClusterConfig.VectorAggregatorConfigMapName,
			)
			if err != nil {
				return "", err
			}
			return s, nil
		}
	}
	return "", nil
}
