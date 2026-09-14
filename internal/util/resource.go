package util

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// QuantityToMB converts a resource quantity to mebibytes.
func QuantityToMB(quantity resource.Quantity) float64 {
	return (float64(quantity.Value() / (1024 * 1024)))
}
