package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/kafka-operator/internal/util"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("QuantityToMB", func() {
	Context("when converting a valid resource.Quantity", func() {
		It("should convert to megabytes correctly", func() {
			// given
			quantity := resource.MustParse("1Gi")

			// when
			result := util.QuantityToMB(quantity)

			// then
			Expect(result).To(BeNumerically("~", 1024, 0.1))
		})
	})

	Context("when handling typical Kubernetes resource quantities", func() {
		It("should handle them correctly", func() {
			// given
			quantity := resource.MustParse("500Mi")

			// when
			result := util.QuantityToMB(quantity)

			// then
			Expect(result).To(BeNumerically("~", 500, 0.1))
		})
	})

	Context("when handling zero quantity", func() {
		It("should handle zero quantity without errors", func() {
			// given
			quantity := resource.MustParse("0")

			// when
			result := util.QuantityToMB(quantity)

			// then
			Expect(result).To(Equal(0.0))
		})
	})

	Context("when handling very large quantities", func() {
		It("should manage them without overflow", func() {
			// given
			quantity := resource.MustParse("1Ti")

			// when
			result := util.QuantityToMB(quantity)

			// then
			Expect(result).To(BeNumerically("~", 1048576, 0.1))
		})
	})

	Context("when handling negative quantities", func() {
		It("should handle negative quantities gracefully", func() {
			// given
			quantity := resource.MustParse("-1Gi")

			// when
			result := util.QuantityToMB(quantity)

			// then
			Expect(result).To(BeNumerically("~", -1024, 0.1))
		})
	})
})
