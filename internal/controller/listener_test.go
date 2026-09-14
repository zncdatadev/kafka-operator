package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/reconciler"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/security"
)

func newTestSecurity(tls *kafkav1alpha1.KafkaTlsSpec, auth []kafkav1alpha1.KafkaAuthenticationSpec) *security.KafkaSecurity {
	return security.NewKafkaSecurity(&kafkav1alpha1.KafkaCluster{
		Spec: kafkav1alpha1.KafkaClusterSpec{
			ClusterConfig: &kafkav1alpha1.ClusterConfigSpec{
				ZookeeperConfigMapName: "zk",
				Tls:                    tls,
				Authentication:         auth,
			},
		},
	})
}

func newTestBuildCtx() *reconciler.RoleGroupBuildContext {
	return &reconciler.RoleGroupBuildContext{
		ClusterName:      "kafkacluster",
		ClusterNamespace: "default",
		RoleName:         kafkav1alpha1.BrokerRoleName,
		RoleGroupName:    "default",
		ResourceName:     "kafkacluster-default",
	}
}

var _ = Describe("Kafka listener config", func() {
	h := NewKafkaRoleGroupHandler(nil)
	brokerCfg := &brokerConfig{
		BrokerListenerClass:    "cluster-internal",
		BootstrapListenerClass: "cluster-internal",
	}

	listenerConfigFor := func(sec *security.KafkaSecurity) *KafkaListenerConfig {
		provisioner := h.buildListenerProvisioner(brokerCfg, "kafkacluster-default-bootstrap")
		cfg, err := GetKafkaListenerConfig(newTestBuildCtx(), sec, provisioner)
		Expect(err).NotTo(HaveOccurred())
		return cfg
	}

	It("exposes plaintext CLIENT and INTERNAL listeners without TLS", func() {
		cfg := listenerConfigFor(newTestSecurity(nil, nil))

		Expect(cfg.ListenersString()).To(Equal("CLIENT://0.0.0.0:9092,INTERNAL://0.0.0.0:19092"))
		Expect(cfg.AdvertisedListenersString()).To(Equal(
			"CLIENT://$(cat /kubedoop/listener/listener-broker/default-address/address):" +
				"$(cat /kubedoop/listener/listener-broker/default-address/ports/kafka)," +
				"INTERNAL://$POD_NAME.kafkacluster-default-headless.default.svc.cluster.local:19092"))
		Expect(cfg.ListenerSecurityProtocolMapString()).To(Equal("CLIENT:PLAINTEXT,INTERNAL:PLAINTEXT"))
	})

	It("uses the secure client port and SSL when server TLS is enabled", func() {
		cfg := listenerConfigFor(newTestSecurity(&kafkav1alpha1.KafkaTlsSpec{
			ServerSecretClass: "tls",
			SSLStorePassword:  "changeit",
		}, nil))

		Expect(cfg.ListenersString()).To(Equal("CLIENT://0.0.0.0:9093,INTERNAL://0.0.0.0:19092"))
		Expect(cfg.ListenerSecurityProtocolMapString()).To(Equal("CLIENT:SSL,INTERNAL:PLAINTEXT"))
		Expect(cfg.AdvertisedListenersString()).To(ContainSubstring("ports/kafka-tls)"))
	})

	It("secures the INTERNAL listener when internal TLS is enabled", func() {
		cfg := listenerConfigFor(newTestSecurity(&kafkav1alpha1.KafkaTlsSpec{
			InternalSecretClass: "tls",
			SSLStorePassword:    "changeit",
		}, nil))

		Expect(cfg.ListenersString()).To(Equal("CLIENT://0.0.0.0:9092,INTERNAL://0.0.0.0:19093"))
		Expect(cfg.ListenerSecurityProtocolMapString()).To(Equal("CLIENT:PLAINTEXT,INTERNAL:SSL"))
	})

	It("adds the BOOTSTRAP listener when Kerberos is enabled", func() {
		cfg := listenerConfigFor(newTestSecurity(
			&kafkav1alpha1.KafkaTlsSpec{ServerSecretClass: "tls", SSLStorePassword: "changeit"},
			[]kafkav1alpha1.KafkaAuthenticationSpec{
				{Kerberos: &kafkav1alpha1.KerberosAuthenticationProviderSpec{KerberosSecretClass: "kerberos"}},
			}))

		Expect(cfg.ListenersString()).To(Equal(
			"CLIENT://0.0.0.0:9093,INTERNAL://0.0.0.0:19093,BOOTSTRAP://0.0.0.0:9095"))
		Expect(cfg.ListenerSecurityProtocolMapString()).To(Equal("CLIENT:SSL,INTERNAL:SSL,BOOTSTRAP:SSL"))
	})
})

var _ = Describe("Bootstrap listener CR", func() {
	It("carries the bootstrap marker label, class and ports", func() {
		sec := newTestSecurity(nil, nil)
		obj := NewBootstrapListener("kafkacluster-default-bootstrap", "default",
			map[string]string{"app.kubernetes.io/instance": "kafkacluster"}, "external-stable", sec)

		l, ok := obj.(*listenerv1alpha1.Listener)
		Expect(ok).To(BeTrue())
		Expect(l.Name).To(Equal("kafkacluster-default-bootstrap"))
		Expect(l.Labels).To(HaveKeyWithValue(LabelListenerBootstrap, LabelValueTrue))
		Expect(l.Labels).To(HaveKeyWithValue("app.kubernetes.io/instance", "kafkacluster"))
		Expect(l.Spec.ClassName).To(Equal("external-stable"))
		Expect(l.Spec.PublishNotReadyAddresses).To(HaveValue(BeTrue()))
		Expect(l.Spec.Ports).To(HaveLen(2))
		Expect(l.Spec.Ports[0].Name).To(Equal("kafka"))
		Expect(l.Spec.Ports[0].Port).To(Equal(int32(9092)))
	})
})

var _ = Describe("Broker config resolution", func() {
	It("falls back to defaults when nothing is set", func() {
		cfg := resolveBrokerConfig(&kafkav1alpha1.KafkaCluster{}, "default")
		Expect(cfg.BrokerListenerClass).To(Equal("cluster-internal"))
		Expect(cfg.BootstrapListenerClass).To(Equal("cluster-internal"))
		Expect(cfg.RequestedSecretLifeTime).To(Equal("1d"))
	})

	It("prefers role group config over role config", func() {
		cr := &kafkav1alpha1.KafkaCluster{
			Spec: kafkav1alpha1.KafkaClusterSpec{
				Brokers: &kafkav1alpha1.BrokersSpec{
					Config: &kafkav1alpha1.BrokersConfigSpec{
						BrokerListenerClass:     "external-unstable",
						RequestedSecretLifeTime: "7d",
					},
					RoleGroups: map[string]*kafkav1alpha1.BrokersRoleGroupSpec{
						"default": {
							Config: &kafkav1alpha1.BrokersConfigSpec{
								BrokerListenerClass: "external-stable",
							},
						},
					},
				},
			},
		}
		cfg := resolveBrokerConfig(cr, "default")
		Expect(cfg.BrokerListenerClass).To(Equal("external-stable"))
		Expect(cfg.BootstrapListenerClass).To(Equal("cluster-internal"))
		Expect(cfg.RequestedSecretLifeTime).To(Equal("7d"))

		other := resolveBrokerConfig(cr, "other")
		Expect(other.BrokerListenerClass).To(Equal("external-unstable"))
	})
})
