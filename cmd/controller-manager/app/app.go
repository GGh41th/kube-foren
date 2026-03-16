/*
Copyright 2026.

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

package app

import (
	"crypto/tls"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubeforenv1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	"github.com/ggh41th/kubeforen/cmd/controller-manager/app/options"
	"github.com/ggh41th/kubeforen/internal/controller"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	// +kubebuilder:scaffold:imports
)

const (
	controllerName = "checkpoint-controller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kubeforenv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func NewControllerManagerCommand() *cobra.Command {
	opts := &options.ControllerManagerOptions{}
	fs := pflag.NewFlagSet(controllerName, pflag.ExitOnError)
	opts.Addflags(fs)

	cmd := &cobra.Command{
		Use:   controllerName,
		Short: "Controller manager for rbac-controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			// parse flags from args
			if err := fs.Parse(args); err != nil {
				return err
			}
			return runControllerManager(opts)
		},
	}
	cmd.Flags().AddFlagSet(fs)
	return cmd
}

func runControllerManager(opts *options.ControllerManagerOptions) error {

	var tlsOpts []func(*tls.Config)
	logOpts := zap.Options{
		Development: true,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logOpts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !opts.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress:   opts.MetricsAddr,
		SecureServing: opts.SecureMetrics,
		TLSOpts:       tlsOpts,
	}
	// enable authN/authZ for metrics endpoint
	if opts.SecureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	// [TODO: Integrate with cert-manager]
	if len(opts.MetricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", opts.MetricsCertPath, "metrics-cert-name", opts.MetricsCertName, "metrics-cert-key", opts.MetricsCertKey)

		metricsServerOptions.CertDir = opts.MetricsCertPath
		metricsServerOptions.CertName = opts.MetricsCertName
		metricsServerOptions.KeyName = opts.MetricsCertKey
	}

	electionName := controllerName
	cfg, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "Failed to get kubeconfig")
	}
	mgr, err := ctrl.NewManager(cfg, manager.Options{
		Metrics:          metricsServerOptions,
		LeaderElection:   opts.EnableLeaderElection,
		LeaderElectionID: electionName,
		PprofBindAddress: opts.ProbeBindAddress,
	})

	if err != nil {
		setupLog.Error(err, "Failed to create manager")
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "error adding healthz checker")
		return err
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "error adding Readyz checker")
		return err
	}

	if err := kubeforenv1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to register scheme", "api", kubeforenv1.GroupVersion.String())
		return err
	}

	kubeclient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	if err := (&controller.CheckPointReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		KubeClient: kubeclient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to setup controller with manager")
		return err
	}

	rootCtx := signals.SetupSignalHandler()

	if err := mgr.Start(rootCtx); err != nil {
		setupLog.Error(err, "unable to start manager")
	}
	return nil
}
