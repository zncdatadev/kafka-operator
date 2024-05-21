package common

import (
	"context"
	"fmt"

	"github.com/zncdatadev/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapType interface {
	ResourceBuilder
	ConfigurationOverride
}

type ConfigMapBuilder struct {
	Name             string
	Namespace        string
	Labels           map[string]string
	ConfigGenerators []ConfigGenerator
}

func (c *ConfigMapBuilder) Build() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    c.Labels,
		},
		Data: GenerateAll(c.ConfigGenerators),
	}
}

type ConfigType string

const (
	Xml        ConfigType = "xml"
	Properties ConfigType = "properties"
)

// OverrideConfigFileContent override config file content
// if we need to override config file content, we should use this function
// if field exist in current configMap, we need to override it
// if field not exist in current configMap, we need to append it
func OverrideConfigFileContent(current string, override map[string]string, configType ConfigType) string {
	switch configType {
	case Xml:
		return util.OverrideXmlContent(current, override)
	case Properties:
		overrideParis := make([]util.NameValuePair, 0)
		for k, v := range override {
			overrideParis = append(overrideParis, util.NameValuePair{
				Name:  k,
				Value: v,
			})
		}
		content, err := util.OverridePropertiesFileContent(current, overrideParis)
		if err != nil {
			return ""
		}
		return content
	default:
		panic(fmt.Sprintf("unknown config type: %s", configType))
	}
}

// ConfigGenerator generate config
// we can use this interface to generate config content
// and use GenerateAll function to generate configMap data
type ConfigGenerator interface {
	Generate() string
	FileName() string
}

func GenerateAll(confGenerator []ConfigGenerator) map[string]string {
	data := make(map[string]string)
	for _, generator := range confGenerator {
		if generator.Generate() != "" {
			data[generator.FileName()] = generator.Generate()
		}
	}
	return data
}

type SecurityProtocol string

const (
	Plaintext SecurityProtocol = "PLAINTEXT"
	Ssl       SecurityProtocol = "SSL"
	SaslSsl   SecurityProtocol = "SASL_SSL"
	SaslPlain SecurityProtocol = "SASL_PLAINTEXT"
)

// GeneralConfigMapReconciler general config map reconciler generator
// it can be used to generate config map reconciler for simple config map
// parameters:
// 1. resourceBuilerFunc: a function to create a new resource
type GeneralConfigMapReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	resourceBuilderFunc       func() (client.Object, error)
	configurationOverrideFunc func() error
}

// NewGeneralConfigMap new a GeneralConfigMapReconciler
func NewGeneralConfigMap[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	resourceBuilderFunc func() (client.Object, error),
	configurationOverrideFunc func() error,

) *GeneralConfigMapReconciler[T, G] {
	return &GeneralConfigMapReconciler[T, G]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg),
		resourceBuilderFunc:       resourceBuilderFunc,
		configurationOverrideFunc: configurationOverrideFunc,
	}
}

// Build implements the ResourceBuilder interface
func (c *GeneralConfigMapReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	return c.resourceBuilderFunc()
}

// ConfigurationOverride implement ConfigurationOverride interface
func (c *GeneralConfigMapReconciler[T, G]) ConfigurationOverride(resource client.Object) {
	if c.configurationOverrideFunc != nil {
		err := c.configurationOverrideFunc()
		if err != nil {
			return
		}
	}
}
