package config

import (
	"math"
	"strings"

	"golang.org/x/exp/maps"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
)

// const VECTOR_AGGREGATOR_CM_ENTRY: &str = "ADDRESS";
// const CONSOLE_CONVERSION_PATTERN: &str = "[%d] %p %m (%c)%n";
const ConsoleConversionPattern = "[%d] %p %m (%c)%n"
const DefalutLoggerLevel = "INFO"
const KafkaLogFile = "kafka.log4j.xml"
const MaxKafkaLogFilesSizeInMiB int32 = 10 // 10 MiB
const log4jTemplate = `log4j.rootLogger={{.RootLogLevel}}, CONSOLE, FILE

log4j.appender.CONSOLE=org.apache.log4j.ConsoleAppender
log4j.appender.CONSOLE.Threshold={{.ConsoleLogLevel}}
log4j.appender.CONSOLE.layout=org.apache.log4j.PatternLayout
log4j.appender.CONSOLE.layout.ConversionPattern={{.ConsoleConversionPattern}}

log4j.appender.FILE=org.apache.log4j.RollingFileAppender
log4j.appender.FILE.Threshold={{.FileLogLevel}}
log4j.appender.FILE.File={{.LogDir}}/{{.LogFile}}
log4j.appender.FILE.MaxFileSize={{.MaxLogFileSizeInMiB}}MB
log4j.appender.FILE.MaxBackupIndex={{.NumberOfArchivedLogFiles}}
log4j.appender.FILE.layout=org.apache.log4j.xml.XMLLayout

{{.Loggers}}
`

// log4j.properties

// Log4jConfGenerator kafka log4j conf generator
type Log4jConfGenerator struct {
	LoggingSpec *kafkav1alpha1.LoggingConfigSpec
	Container   string
}

type Log4jConfig struct {
	RootLogLevel             string
	ConsoleLogLevel          string
	ConsoleConversionPattern string
	FileLogLevel             string
	LogDir                   string
	LogFile                  string
	MaxLogFileSizeInMiB      int
	NumberOfArchivedLogFiles int
	Loggers                  string
}

func (g *Log4jConfGenerator) Generate() (string, error) {
	parser := config.TemplateParser{
		Template: log4jTemplate,
		Value:    g.Config(),
	}
	if data, err := parser.Parse(); err != nil {
		return "", err
	} else {
		return data, nil
	}
}

// create log4j config
func (g *Log4jConfGenerator) Config() *Log4jConfig {
	rootLogLevel := DefalutLoggerLevel
	consoleLogLevel := DefalutLoggerLevel
	fileLogLevel := DefalutLoggerLevel
	numberOfArchivedLogFiles := 1
	maxLogFileSizeInMiB := 10.0
	var loggerNames string
	if g.LoggingSpec != nil {
		loggersSpec := g.LoggingSpec.Loggers
		consoleLogSpec := g.LoggingSpec.Console
		fileLogSpec := g.LoggingSpec.File
		// if console or file level is not empty, use it, otherwise use default
		consoleLogLevel = GetLoggerLevel(consoleLogSpec.Level != "", func() string { return consoleLogSpec.Level }, DefalutLoggerLevel)
		fileLogLevel = GetLoggerLevel(fileLogSpec.Level != "", func() string { return fileLogSpec.Level }, DefalutLoggerLevel)
		// extract root log level and logger names
		if len(loggersSpec) != 0 {
			// check root logger exists in loggers, if not, use default.
			if _, ok := loggersSpec[kafkav1alpha1.RootLogger]; ok {
				definedRootLogLevel := loggersSpec[kafkav1alpha1.RootLogger].Level
				if definedRootLogLevel != "" {
					rootLogLevel = definedRootLogLevel
				}
			}
			cloneLoggers := maps.Clone(loggersSpec)
			// Deletes the logger associated with the RootLogger key from the cloneLoggers map.
			// If the cloneLoggers map is not empty after deletion, concatenates the keys of the remaining loggers
			// with a comma and assigns the result to loggerNames.
			maps.DeleteFunc(cloneLoggers, func(key string, value *kafkav1alpha1.LogLevelSpec) bool { return key == kafkav1alpha1.RootLogger })
			if len(cloneLoggers) != 0 {
				loggerNames = strings.Join(maps.Keys(cloneLoggers), ",")
			}
		}
		// compute max log file size
		maxLogFileSizeInMiB = math.Max(1, float64(MaxKafkaLogFilesSizeInMiB)/(1+float64(numberOfArchivedLogFiles)))
	}
	return &Log4jConfig{
		RootLogLevel:             rootLogLevel,
		ConsoleConversionPattern: ConsoleConversionPattern,
		ConsoleLogLevel:          consoleLogLevel,
		FileLogLevel:             fileLogLevel,
		LogDir:                   kafkav1alpha1.KubedoopLogDir + "/" + strings.ToLower(g.Container),
		LogFile:                  KafkaLogFile,
		MaxLogFileSizeInMiB:      int(maxLogFileSizeInMiB),
		NumberOfArchivedLogFiles: numberOfArchivedLogFiles,
		Loggers:                  loggerNames,
	}
}

func (g *Log4jConfGenerator) FileName() string {
	return kafkav1alpha1.Log4jFileName
}

func GetLoggerLevel(condition bool, trueValFunc func() string, defaultVal string) string {
	if condition {
		trueVal := trueValFunc()
		if strings.TrimSpace(trueVal) != "" {
			return trueVal
		}
	}
	return defaultVal
}
