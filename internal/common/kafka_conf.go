package common

import (
	"fmt"
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
)

// log4j.properties

// Log4jConfGenerator kafka log4j conf generator
type Log4jConfGenerator struct {
}

func (g *Log4jConfGenerator) Generate() string {
	return originLog4jProperties
}

func (g *Log4jConfGenerator) FileName() string {
	return v1alpha1.Log4jFileName
}

// security.properties

// SecurityConfGenerator kafka security conf generator
type SecurityConfGenerator struct {
}

func (g *SecurityConfGenerator) Generate() string {
	return `networkaddress.cache.negative.ttl=0
networkaddress.cache.ttl=30`
}

func (g *SecurityConfGenerator) FileName() string {
	return v1alpha1.SecurityFileName
}

//server.properties

// KafkaConfGenerator kafka config generator
type KafkaConfGenerator struct {
	SslSpec *v1alpha1.SslSpec
}

const serverProperties = `
controlled.shutdown.enable=true
zookeeper.connection.timeout.ms=18000
`

//controlled.shutdown.enable=true
//inter.broker.listener.name=INTERNAL
//listener.name.internal.ssl.client.auth=required
//listener.name.internal.ssl.keystore.location=/stackable/tls_keystore_internal/keystore.p12
//listener.name.internal.ssl.keystore.password=
//listener.name.internal.ssl.keystore.type=PKCS12
//listener.name.internal.ssl.truststore.location=/stackable/tls_keystore_internal/truststore.p12
//listener.name.internal.ssl.truststore.password=
//listener.name.internal.ssl.truststore.type=PKCS12
//log.dirs=/stackable/data/topicdata
//zookeeper.connect=localhost\:2181
//zookeeper.connection.timeout.ms=18000

func (k *KafkaConfGenerator) Generate() string {
	return serverProperties + "\n" +
		"listeners=" + k.Listeners() + "\n" +
		"inter.broker.listener.name=" + k.InterBrokerListenerName() + "\n" +
		k.listenerSslProperties(k.SslSpec) + "\n" +
		"log.dirs=" + k.DataDir() + "\n" +
		//k.zkConnections() + "\n" +
		"listener.security.protocol.map=" + k.ListenerSecurityProtocolMap(k.SslSpec)
}

func (k *KafkaConfGenerator) FileName() string {
	return v1alpha1.ServerFileName
}

const interListerName = INTERNAL

// InterBrokerListenerName inter broker listener name
func (k *KafkaConfGenerator) InterBrokerListenerName() string {
	return string(interListerName)
}

// Listeners listeners
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listeners=CLIENT://:9092,ClUSTER://:19092,EXTERNAL://:{nodePort}
func (k *KafkaConfGenerator) Listeners() string {
	return fmt.Sprintf("%s://:%d,%s://:%d", CLIENT, v1alpha1.KafkaClientPort,
		INTERNAL, v1alpha1.InternalPort)
}

// ListenerSecurityProtocolMap
// contains 3 listeners: CLIENT, CLUSTER, EXTERNAL
// listener.security.protocol.map=CLIENT:PLAINTEXT,CLUSTER:PLAINTEXT,EXTERNAL:PLAINTEXT
func (k *KafkaConfGenerator) ListenerSecurityProtocolMap(sslSpec *v1alpha1.SslSpec) string {
	var protocol SecurityProtocol
	if SslEnabled(sslSpec) {
		protocol = Ssl
	} else {
		protocol = Plaintext
	}
	return fmt.Sprintf("%s:PLAINTEXT,%s:%s", CLIENT, INTERNAL, string(protocol))
}

// DataDir data dir
// ex: log.dirs=/var/lib/zookeeper/data
func (k *KafkaConfGenerator) DataDir() string {
	return DataMountPath + "/data/topicdata"
}

// listener ssl properties
// ex:
// ssl.keystore.location=/opt/kafka/config/keystore.jks
// ssl.keystore.password=${env.KAFKA_SSL_KEYSTORE_PASSWORD}
// ssl.key.password=${env.KAFKA_SSL_KEY_PASSWORD}
// ssl.truststore.location=/opt/kafka/config/truststore.jks
// ssl.truststore.password=${env.KAFKA_SSL_TRUSTSTORE_PASSWORD}
func (k *KafkaConfGenerator) listenerSslProperties(sslSpec *v1alpha1.SslSpec) string {
	if SslEnabled(sslSpec) {
		return "ssl.keystore.location=" + TlsKeystoreMountPath + "/keystore.jks\n" +
			"ssl.keystore.password=" + sslSpec.JksPassword + "\n" +
			"ssl.keystore.type=JKS\n" +
			"ssl.truststore.location=" + TlsKeystoreMountPath + "/truststore.jks\n" +
			"ssl.truststore.password=" + sslSpec.JksPassword + "\n" +
			"ssl.truststore.type=JKS"
	}
	return ""
}

func TlsKeystoreInternalVolumeName() string {
	return "tls-keystore-internal"
}

const TlsDir = "tls-keystore-internal"

const (
	FetchNodePort ContainerComponent = "fetch-node-port"
	Kafka         ContainerComponent = "kafka"
)

// mount
const (
	TlsKeystoreMountPath          = "/bitnami/kafka/config/certs"
	SecretTlsPkcs12KeyStorePath   = TlsKeystoreMountPath + "/keystore.p12"
	SecretTlsPkcs12TrustStorePath = TlsKeystoreMountPath + "/truststore.p12"
	KafkaTlsJksKeyStorePath       = TlsKeystoreMountPath + "/kafka.keystore.jks"
	KafkaTlsJks12TrustStorePath   = TlsKeystoreMountPath + "/kafka.truststore.jks"
	DataMountPath                 = "/bitnami/kafka"
	LogMountPath                  = "/opt/bitnami/kafka/logs"
	ConfigMountPath               = "/opt/bitnami/kafka/config/server.properties"
	Log4jMountPath                = "/opt/bitnami/kafka/config/log4j.properties"

	NodePortMountPath = "/zncdata/tmp"
	NodePortFileName  = "kafka_nodeport"

	ConfigMapMountPath    = "/configmaps"
	ServerConfigMountPath = "/config"
)

// env

const (
	EnvZookeeperConnections        = "KAFKA_CFG_ZOOKEEPER_CONNECT"
	EnvNode                        = "ENV_NODE"
	EnvNodePort                    = "NODE_PORT"
	EnvPodName                     = "POD_NAME"
	EnvListeners                   = "KAFKA_CFG_LISTENERS"
	EnvAdvertisedListeners         = "KAFKA_CFG_ADVERTISED_LISTENERS"
	EnvListenerSecurityProtocolMap = "KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"
	//ssl
	EnvCertificatePass       = "KAFKA_CERTIFICATE_PASSWORD"
	EnvTlsTrustStoreLocation = "KAFKA_TLS_TRUSTSTORE_FILE"
	EnvTlsType               = "KAFKA_TLS_TYPE"
	EnvKafkaCertDir          = "KAFKA_CERTS_DIR"
)

const originLog4jProperties = `
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Unspecified loggers and loggers with additivity=true output to server.log and stdout
# Note that INFO only applies to unspecified loggers, the log level of the child logger is used otherwise
log4j.rootLogger=INFO, stdout, kafkaAppender

log4j.appender.stdout=org.apache.log4j.ConsoleAppender
log4j.appender.stdout.layout=org.apache.log4j.PatternLayout
log4j.appender.stdout.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.kafkaAppender=org.apache.log4j.ConsoleAppender
log4j.appender.kafkaAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.kafkaAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.stateChangeAppender=org.apache.log4j.ConsoleAppender
log4j.appender.stateChangeAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.stateChangeAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.requestAppender=org.apache.log4j.ConsoleAppender
log4j.appender.requestAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.requestAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.cleanerAppender=org.apache.log4j.ConsoleAppender
log4j.appender.cleanerAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.cleanerAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.controllerAppender=org.apache.log4j.ConsoleAppender
log4j.appender.controllerAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.controllerAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

log4j.appender.authorizerAppender=org.apache.log4j.ConsoleAppender
log4j.appender.authorizerAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.authorizerAppender.layout.ConversionPattern=[%d] %p %m (%c)%n

# Change the line below to adjust ZK client logging
log4j.logger.org.apache.zookeeper=INFO

# Change the two lines below to adjust the general broker logging level (output to server.log and stdout)
log4j.logger.kafka=INFO
log4j.logger.org.apache.kafka=INFO

# Change to DEBUG or TRACE to enable request logging
log4j.logger.kafka.request.logger=WARN, requestAppender
log4j.additivity.kafka.request.logger=false

# Uncomment the lines below and change log4j.logger.kafka.network.RequestChannel$ to TRACE for additional output
# related to the handling of requests
#log4j.logger.kafka.network.Processor=TRACE, requestAppender
#log4j.logger.kafka.server.KafkaApis=TRACE, requestAppender
#log4j.additivity.kafka.server.KafkaApis=false
log4j.logger.kafka.network.RequestChannel$=WARN, requestAppender
log4j.additivity.kafka.network.RequestChannel$=false

# Change the line below to adjust KRaft mode controller logging
log4j.logger.org.apache.kafka.controller=INFO, controllerAppender
log4j.additivity.org.apache.kafka.controller=false

# Change the line below to adjust ZK mode controller logging
log4j.logger.kafka.controller=TRACE, controllerAppender
log4j.additivity.kafka.controller=false

log4j.logger.kafka.log.LogCleaner=INFO, cleanerAppender
log4j.additivity.kafka.log.LogCleaner=false

log4j.logger.state.change.logger=INFO, stateChangeAppender
log4j.additivity.state.change.logger=false

# Access denials are logged at INFO level, change to DEBUG to also log allowed accesses
log4j.logger.kafka.authorizer.logger=INFO, authorizerAppender
log4j.additivity.kafka.authorizer.logger=false
log4j.appender.stdout.Threshold=OFF
`
