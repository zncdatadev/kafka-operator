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

type RoleLoggingDataBuilder interface {
	MakeContainerLogData() map[string]string
}

type OverrideExistLogging interface {
	OverrideExist(exist *corev1.ConfigMap)
}

type BaseRoleLoggingDataBuilder struct {
	Role Role
}

func (b *BaseRoleLoggingDataBuilder) MakeContainerLogData() map[string]string {
	// todo: make data for multi container, and support multi log framework by using LoggingPluggingDataBuilder interface, such as log4j, logback, etc
	return nil
}

type LoggingRecociler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	RoleLoggingDataBuilder RoleLoggingDataBuilder
	role                   Role
	InstanceGetter         InstanceAttributes
	ConfigmapName          string
}

// NewLoggingReconciler new logging reconcile
func NewLoggingReconciler[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,
	logDataBuilder RoleLoggingDataBuilder,
	role Role,
	configmapName string,
) *LoggingRecociler[T, any] {
	return &LoggingRecociler[T, any]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		RoleLoggingDataBuilder: logDataBuilder,
		role:                   role,
		ConfigmapName:          configmapName,
	}
}

// Build log4j config map
func (l *LoggingRecociler[T, G]) Build(_ context.Context) (client.Object, error) {
	cmData := l.RoleLoggingDataBuilder.MakeContainerLogData()
	if len(cmData) == 0 {
		return nil, nil
	}
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      l.ConfigmapName,
			Namespace: l.Instance.GetNamespace(),
			Labels:    l.Labels,
		},
		Data: cmData,
	}
	return obj, nil
}

// OverrideExistLoggingRecociler override exist logging config reconciler
// if log properties exist in some configmap, we need to override it
type OverrideExistLoggingRecociler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	RoleLoggingDataBuilder RoleLoggingDataBuilder
}

// NewOverrideExistLoggingRecociler new OverrideExistLoggingReconcile
func NewOverrideExistLoggingRecociler[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,
	logDataBuilder RoleLoggingDataBuilder,
) *OverrideExistLoggingRecociler[T, any] {
	return &OverrideExistLoggingRecociler[T, any]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		RoleLoggingDataBuilder: logDataBuilder,
	}
}

// OverrideExist override exist logging config
func (l *OverrideExistLoggingRecociler[T, G]) OverrideExist(exist *corev1.ConfigMap) {
	exist.Data = l.RoleLoggingDataBuilder.MakeContainerLogData()
}

// LoggingPluggingDataBuilder all logging data builder abstract interface
// for now, only support log4j
// todo: support other log framework, such as logback, log4j2 etc
type LoggingPluggingDataBuilder interface {
	MakeContainerLogProperties(origin string) string

	MakeDefaultFileAppenderProperties() map[string]string
}

type LogBuilderLoggers struct {
	logger string
	level  string
}

type LogBuilderAppender struct {
	appenderName       string
	level              string
	defaultLogLocation string
}

type Log4jLoggingDataBuilder struct {
	Loggers []LogBuilderLoggers
	Console *LogBuilderAppender
	File    *LogBuilderAppender
}

// MakeContainerLogProperties make log4j properties
func (l *Log4jLoggingDataBuilder) MakeContainerLogProperties(origin string) string {
	content, err := util.OverridePropertiesFileContent(origin, l.MakeOverrideLoggerProperties())
	if err != nil {
		return origin
	}
	return content
}

// MakeOverrideLoggerProperties get override logger properties
// do works below:
// 1. make custom loggers properties
// 2. make console logger properties
// 3. make file appender logger properties
// 4. merge all the properties
func (l *Log4jLoggingDataBuilder) MakeOverrideLoggerProperties() []util.NameValuePair {
	loggers := l.makeCustomLoggersProperties()
	console := l.makeConsoleLoggerProperties()
	file := l.makeFileLoggerProperties()
	properties := make([]util.NameValuePair, 0)
	properties = append(properties, loggers...)
	properties = append(properties, console...)
	properties = append(properties, file...)
	return properties
}

func (l *Log4jLoggingDataBuilder) makeCustomLoggersProperties() []util.NameValuePair {
	if l.Loggers == nil {
		return nil
	}
	properties := make([]util.NameValuePair, 0)
	for _, logger := range l.Loggers {
		properties = append(properties, util.NameValuePair{
			Name:  "log4j.logger." + logger.logger,
			Value: logger.level,
		})
	}
	return properties
}

// make console logger properties
// change console appender logger level:  "log4j.appender.CONSOLE.Threshold=INFO"
func (l *Log4jLoggingDataBuilder) makeConsoleLoggerProperties() []util.NameValuePair {
	if l.Console == nil {
		return nil
	}
	properties := make([]util.NameValuePair, 0)
	key := fmt.Sprintf("log4j.appender.%s.Threshold", l.Console.appenderName)
	properties = append(properties, util.NameValuePair{
		Name:  key,
		Value: l.Console.level,
	})
	return properties
}

// make file appender logger properties
// change file appender logger level: "log4j.appender.FILE.Threshold=INFO"
func (l *Log4jLoggingDataBuilder) makeFileLoggerProperties() []util.NameValuePair {
	if l.File == nil {
		return nil
	}
	fileAppender := l.File.appenderName
	properties := make([]util.NameValuePair, 0)
	// if file appender not exists, create new default one
	if fileAppender == NoneAppender {
		fileAppender = DefaultFileAppender
		properties = l.MakeDefaultFileAppenderProperties(fileAppender)
	} else {
		key := fmt.Sprintf("log4j.appender.%s.Threshold", l.File.appenderName)
		properties = append(properties, util.NameValuePair{Name: key, Value: l.File.level})
	}
	return properties
}

func (l *Log4jLoggingDataBuilder) MakeDefaultFileAppenderProperties(fileAppender string) []util.NameValuePair {
	prefix := fmt.Sprintf("log4j.appender.%s", fileAppender)
	properties := make([]util.NameValuePair, 0)
	properties = append(properties, util.NameValuePair{Name: prefix, Value: "org.apache.log4j.RollingFileAppender"})
	properties = append(properties, util.NameValuePair{Name: prefix + ".Threshold", Value: l.Console.level})
	properties = append(properties, util.NameValuePair{Name: prefix + ".MaxFileSize", Value: "5MB"})
	properties = append(properties, util.NameValuePair{Name: prefix + ".MaxBackupIndex", Value: "1"})
	properties = append(properties, util.NameValuePair{Name: prefix + ".layout", Value: "org.apache.log4j.PatternLayout"})
	properties = append(properties, util.NameValuePair{Name: prefix + ".layout.ConversionPattern", Value: "%d{ISO8601} %-5p %c{2} (%F:%M(%L)) - %m%n"})
	properties = append(properties, util.NameValuePair{Name: prefix + ".File", Value: l.File.defaultLogLocation})
	return properties
}
