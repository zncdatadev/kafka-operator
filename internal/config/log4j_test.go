package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/config"
)

var _ = Describe("Log4jConfGenerator", func() {
	Context("when loggingSpec is nil", func() {
		It("should generate Log4jConfig with default values", func() {
			// given
			generator := &config.Log4jConfGenerator{
				LoggingSpec: nil,
			}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
			Expect(log4jConfig.RootLogLevel).To(Equal(config.DefalutLoggerLevel))
			Expect(log4jConfig.ConsoleLogLevel).To(Equal(config.DefalutLoggerLevel))
			Expect(log4jConfig.FileLogLevel).To(Equal(config.DefalutLoggerLevel))
			Expect(log4jConfig.NumberOfArchivedLogFiles).To(Equal(1))
			Expect(log4jConfig.MaxLogFileSizeInMiB).To(Equal(10))
		})

		It("should handle nil loggingSpec without errors", func() {
			// given
			generator := &config.Log4jConfGenerator{LoggingSpec: nil}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
		})
	})

	Context("when console log level is specified", func() {
		It("should generate Log4jConfig with specified console log level", func() {
			// given
			loggingSpec := &v1alpha1.LoggingConfigSpec{
				Console: &v1alpha1.LogLevelSpec{Level: "INFO"},
			}
			generator := &config.Log4jConfGenerator{LoggingSpec: loggingSpec}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
			Expect(log4jConfig.ConsoleLogLevel).To(Equal("INFO"))
		})
	})

	Context("when file log level is specified", func() {
		It("should generate Log4jConfig with specified file log level", func() {
			// given
			loggingSpec := &v1alpha1.LoggingConfigSpec{
				File: &v1alpha1.LogLevelSpec{Level: "DEBUG"},
			}
			generator := &config.Log4jConfGenerator{LoggingSpec: loggingSpec}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
			Expect(log4jConfig.FileLogLevel).To(Equal("DEBUG"))
		})
	})

	Context("when loggersSpec is empty", func() {
		It("should handle empty loggersSpec without errors", func() {
			// given
			loggingSpec := &v1alpha1.LoggingConfigSpec{
				Loggers: map[string]*v1alpha1.LogLevelSpec{},
			}
			generator := &config.Log4jConfGenerator{LoggingSpec: loggingSpec}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
		})
	})

	Context("when root logger is missing in loggersSpec", func() {
		It("should handle missing root logger in loggersSpec correctly", func() {
			// given
			loggingSpec := &v1alpha1.LoggingConfigSpec{
				Loggers: map[string]*v1alpha1.LogLevelSpec{
					"appLogger": {Level: "WARN"},
				},
			}
			generator := &config.Log4jConfGenerator{LoggingSpec: loggingSpec}

			// when
			log4jConfig := generator.Config()

			// then
			Expect(log4jConfig).NotTo(BeNil())
			Expect(log4jConfig.RootLogLevel).To(Equal(config.DefalutLoggerLevel))
		})
	})

	Context("when computing max log file size based on number of archived log files", func() {
		It("should compute max log file size correctly based on number of archived log files", func() {
			// given
			g := &config.Log4jConfGenerator{
				LoggingSpec: &v1alpha1.LoggingConfigSpec{
					Loggers: map[string]*v1alpha1.LogLevelSpec{
						v1alpha1.RootLogger: {
							Level: "INFO",
						},
					},
					Console: &v1alpha1.LogLevelSpec{
						Level: "DEBUG",
					},
					File: &v1alpha1.LogLevelSpec{
						Level: "ERROR",
					},
				},
				Container: "ContainerName",
			}

			// when
			result := g.Config()

			// then
			Expect(result.MaxLogFileSizeInMiB).To(Equal(5)) // Expected value based on the computation
		})
	})
})
