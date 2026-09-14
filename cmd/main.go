/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	opcommon "github.com/zncdatadev/operator-go/pkg/common"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/controller"
	"github.com/zncdatadev/kafka-operator/internal/util/version"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kafkav1alpha1.AddToScheme(scheme))
	// The bootstrap Listener CRs are applied through the GenericReconciler, so the type
	// must be registered in the manager scheme.
	utilruntime.Must(listenerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var showVersion bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if showVersion {
		fmt.Println(version.NewAppInfo("kafka-operator").String())
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServerOptions := webhook.Options{
		TLSOpts: tlsOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)
		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)
		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		WebhookServer:          webhookServer,
		// Keep the pre-refactor Lease name: changing it would let old and new pods each
		// hold "leadership" on different Leases during a rolling upgrade (split-brain).
		LeaderElectionID: "6e8ac606.kubedoop.dev",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup KafkaCluster controller using GenericReconciler
	kafkaHandler := controller.NewKafkaRoleGroupHandler(mgr.GetScheme())

	// Extension registry is per-CR-type and owned by exactly one reconciler; the
	// discovery extension publishes the discovery ConfigMaps (bootstrap servers
	// aggregated from the bootstrap Listeners).
	extensionRegistry := opcommon.NewExtensionRegistry[*kafkav1alpha1.KafkaCluster]()
	extensionRegistry.RegisterClusterExtension(controller.NewDiscoveryExtension(mgr.GetScheme()))

	kafkaReconciler, err := reconciler.NewGenericReconciler(
		&reconciler.GenericReconcilerConfig[*kafkav1alpha1.KafkaCluster]{
			Client: mgr.GetClient(),
			// Uncached: used to refresh the resourceVersion after a conflicting status
			// write, which the informer cache is by definition too stale to serve.
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
			// operator-go's Recorder field is the (deprecated) record.EventRecorder; the
			// replacement GetEventRecorder returns the incompatible events.EventRecorder.
			Recorder:         mgr.GetEventRecorderFor("kafka-cluster-controller"), //nolint:staticcheck
			RoleGroupHandler: kafkaHandler,
			// The handler also declares the broker role, once per reconcile pass with the
			// cr in hand — ports, primary container name, probes, log producers, config
			// defaults. The workload ServiceAccount is framework-derived
			// ("kafkacluster-<cluster>"); kafka pods call no Kubernetes API, so no
			// WorkloadRBACRules.
			RoleProvider: kafkaHandler,
			// Kafka's derived config (default config files, heap from the effective
			// memory limit) flows through the merge pipeline as the lowest layer; user
			// overrides always win.
			RoleGroupResolver: reconciler.RoleGroupResolverFunc[*kafkav1alpha1.KafkaCluster](
				controller.ResolveRoleGroup),
			// Read every reconcile, so an operator upgrade moves existing clusters onto
			// the co-released product image
			// ("{repo}/kafka:{productVersion}-kubedoop{operator build version}").
			ImageResolution: reconciler.ImageResolution{
				ProductName: kafkav1alpha1.DefaultProductName,
				Defaults: commonsv1alpha1.ImageSpec{
					Repo:            kafkav1alpha1.DefaultRepository,
					ProductVersion:  kafkav1alpha1.DefaultProductVersion,
					KubedoopVersion: version.BuildVersion,
				},
			},
			ExtensionRegistry: extensionRegistry,
			Prototype:         &kafkav1alpha1.KafkaCluster{},
		})
	if err != nil {
		setupLog.Error(err, "unable to create GenericReconciler", "controller", "KafkaCluster")
		os.Exit(1)
	}
	// ExtraOwns gives the bootstrap Listener CRs a watch AND registers their kind with the
	// orphan cleaner, which reclaims a removed role group's labelled extras.
	if err := kafkaReconciler.SetupWithManagerOpts(mgr, reconciler.SetupWithManagerOptions{
		ExtraOwns: []ctrlclient.Object{&listenerv1alpha1.Listener{}},
	}); err != nil {
		setupLog.Error(err, "unable to setup controller", "controller", "KafkaCluster")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
