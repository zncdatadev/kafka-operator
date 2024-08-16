package config

import "github.com/zncdatadev/kafka-operator/api/v1alpha1"

// security.properties

// SecurityConfGenerator kafka security conf generator
type SecurityConfGenerator struct {
}

func (g *SecurityConfGenerator) Generate() (string, error) {
	return `networkaddress.cache.negative.ttl=0
networkaddress.cache.ttl=30`, nil
}

func (g *SecurityConfGenerator) FileName() string {
	return v1alpha1.SecurityFileName
}
