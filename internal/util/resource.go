package util

import (
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("util")
)

func ConvertToResourceRequirements(resources *commonsv1alpha1.ResourcesSpec) *corev1.ResourceRequirements {
	var (
		cpuMin      = resource.MustParse("100m")
		cpuMax      = resource.MustParse("500m")
		memoryLimit = resource.MustParse("1Gi")
	)
	if resources != nil {
		if resources.CPU != nil {
			cpuMin = resources.CPU.Min
		}
		if resources.CPU != nil {
			cpuMax = resources.CPU.Max
		}
		if resources.Memory != nil {
			memoryLimit = resources.Memory.Limit
		}
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    cpuMax,
			corev1.ResourceMemory: memoryLimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpuMin,
			corev1.ResourceMemory: memoryLimit,
		},
	}
}

func QuantityToMB(quantity resource.Quantity) float64 {
	return (float64(quantity.Value() / (1024 * 1024)))
}
