package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
)

var _ = Describe("server.properties generation", func() {
	h := NewKafkaRoleGroupHandler(nil)

	It("renders merged config with TLS settings resolved from the provisioner mounts", func() {
		sec := newTestSecurity(&kafkav1alpha1.KafkaTlsSpec{
			ServerSecretClass:   "tls",
			InternalSecretClass: "tls",
			SSLStorePassword:    "changeit",
		}, nil)
		provisioner := h.buildSecretProvisioner(sec, &brokerConfig{RequestedSecretLifeTime: "7d"})

		buildCtx := newTestBuildCtx()
		buildCtx.MergedConfig = &config.MergedConfig{
			ConfigFiles: map[string]map[string]string{
				kafkav1alpha1.ServerFileName: {
					"log.dirs":     "/kubedoop/data/topicdata",
					"user.setting": "custom",
				},
			},
		}

		props := h.generateServerProperties(buildCtx, sec, provisioner)

		Expect(props).To(ContainSubstring(
			"listener.name.client.ssl.keystore.location=/kubedoop/mount/tls-keystore-server/keystore.p12\n"))
		Expect(props).To(ContainSubstring(
			"listener.name.internal.ssl.keystore.location=/kubedoop/mount/tls-keystore-internal/keystore.p12\n"))
		Expect(props).To(ContainSubstring("inter.broker.listener.name=INTERNAL\n"))
		Expect(props).To(ContainSubstring("log.dirs=/kubedoop/data/topicdata\n"))
		Expect(props).To(ContainSubstring("user.setting=custom\n"))
	})

	It("emits sorted, deterministic properties", func() {
		out := toProperties(map[string]string{"b": "2", "a": "1", "c": "3"})
		Expect(out).To(Equal("a=1\nb=2\nc=3\n"))
	})

	It("provides Kafka defaults and the derived heap through the resolver contribution", func() {
		buildCtx := newTestBuildCtx()
		buildCtx.RoleGroupSpec.Config = &commonsv1alpha1.RoleGroupConfigSpec{
			Resources: &commonsv1alpha1.ResourcesSpec{
				Memory: &commonsv1alpha1.MemoryResource{Limit: ptr.To(resource.MustParse("2Gi"))},
			},
		}
		contribution, err := ResolveRoleGroup(context.Background(), nil, nil, buildCtx)
		Expect(err).NotTo(HaveOccurred())
		server := contribution.ConfigOverrides[kafkav1alpha1.ServerFileName]
		Expect(server).To(HaveKeyWithValue("controlled.shutdown.enable", "true"))
		Expect(server).To(HaveKeyWithValue("log.dirs", "/kubedoop/data/topicdata"))
		Expect(contribution.ConfigOverrides[kafkav1alpha1.SecurityFileName]).To(
			HaveKeyWithValue("networkaddress.cache.ttl", "30"))
		// 80% of 2048MB.
		Expect(contribution.EnvVars).To(HaveKeyWithValue(EnvKafkaHeapOpts, "-Xmx1638m"))
	})
})

var _ = Describe("TLS secret provisioner", func() {
	h := NewKafkaRoleGroupHandler(nil)

	It("scopes keystore volumes to the listener volumes and passes the lifetime verbatim", func() {
		sec := newTestSecurity(&kafkav1alpha1.KafkaTlsSpec{
			ServerSecretClass: "tls",
			SSLStorePassword:  "changeit",
		}, nil)
		provisioner := h.buildSecretProvisioner(sec, &brokerConfig{RequestedSecretLifeTime: "7d"})

		volumes := provisioner.Volumes()
		Expect(volumes).To(HaveLen(1))
		annotations := volumes[0].Ephemeral.VolumeClaimTemplate.Annotations
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/class", "tls"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/scope",
			"listener-volume=listener-broker,listener-volume=listener-bootstrap,pod,node"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/autoTlsCertLifetime", "168h0m0s"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/tlsPKCS12Password", "changeit"))
	})

	It("registers a listener-scoped Kerberos keytab volume", func() {
		sec := newTestSecurity(nil, []kafkav1alpha1.KafkaAuthenticationSpec{
			{Kerberos: &kafkav1alpha1.KerberosAuthenticationProviderSpec{KerberosSecretClass: "kerberos"}},
		})
		provisioner := h.buildSecretProvisioner(sec, &brokerConfig{})

		volumes := provisioner.Volumes()
		Expect(volumes).To(HaveLen(1))
		annotations := volumes[0].Ephemeral.VolumeClaimTemplate.Annotations
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/class", "kerberos"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/scope",
			"listener-volume=listener-broker,listener-volume=listener-bootstrap"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/kerberosServiceNames", "kafka"))
		Expect(provisioner.MustPath(kafkav1alpha1.KerberosVolumeName)).To(Equal("/kubedoop/mount/kerberos"))
	})
})

var _ = Describe("Discovery bootstrap servers", func() {
	It("joins listener ingress addresses with the requested port", func() {
		list := &listenerv1alpha1.ListenerList{Items: []listenerv1alpha1.Listener{
			{Status: listenerv1alpha1.ListenerStatus{IngressAddresses: []listenerv1alpha1.IngressAddressSpec{
				{Address: "10.0.0.1", Ports: map[string]int32{"kafka": 9092}},
				{Address: "10.0.0.2", Ports: map[string]int32{"kafka": 9092}},
			}}},
		}}
		Expect(makeBootstrapServers(context.Background(), list, "kafka")).To(Equal("10.0.0.1:9092,10.0.0.2:9092"))
	})

	It("skips listeners without the requested port instead of failing", func() {
		list := &listenerv1alpha1.ListenerList{Items: []listenerv1alpha1.Listener{
			{Status: listenerv1alpha1.ListenerStatus{IngressAddresses: []listenerv1alpha1.IngressAddressSpec{
				{Address: "10.0.0.1", Ports: map[string]int32{"other": 1}},
				{Address: "10.0.0.2", Ports: map[string]int32{"kafka": 9092}},
			}}},
		}}
		Expect(makeBootstrapServers(context.Background(), list, "kafka")).To(Equal("10.0.0.2:9092"))
	})

	It("computes the expected bootstrap listener set from the spec", func() {
		cr := &kafkav1alpha1.KafkaCluster{}
		cr.Name = "kafkacluster"
		cr.Spec.Brokers = &kafkav1alpha1.BrokersSpec{
			RoleGroups: map[string]*kafkav1alpha1.BrokersRoleGroupSpec{
				"default": {}, "extra": {},
			},
		}
		expected := expectedBootstrapListeners(cr)
		Expect(expected).To(HaveKey("kafkacluster-broker-default-bootstrap"))
		Expect(expected).To(HaveKey("kafkacluster-broker-extra-bootstrap"))
		Expect(expected).To(HaveLen(2))
	})
})
