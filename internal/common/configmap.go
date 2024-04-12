package common

import (
	"fmt"
	"github.com/zncdata-labs/kafka-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		content, err := util.OverridePropertiesFileContent(current, override)
		if err != nil {
			return ""
		}
		return content
	default:
		panic(fmt.Sprintf("unknown config type: %s", configType))
	}
	return ""
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
	Same      SecurityProtocol = "SASL_PLAINTEXT"
	SaslSsl   SecurityProtocol = "SASL_SSL"
	SaslPlain SecurityProtocol = "SASL_PLAINTEXT"
)
