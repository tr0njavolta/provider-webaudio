package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	webaudiov1alpha1 "github.com/example/provider-webaudio/apis/webaudio/v1alpha1"
	"github.com/example/provider-webaudio/internal/controller"
	"github.com/example/provider-webaudio/internal/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(webaudiov1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		serverPort           int
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8083", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election.")
	flag.IntVar(&serverPort, "server-port", 9090, "Port for the WebSocket/HTTP server.")
	flag.Parse()

	opts := zap.Options{Development: true}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	srv := server.New(serverPort)
	go srv.Start()
	setupLog.Info("started web server", "port", serverPort)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "webaudio.example.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := controller.NewSequencerReconciler(
		mgr.GetClient(), mgr.GetScheme(), srv.Hub,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create Sequencer controller")
		os.Exit(1)
	}

	if err := (&controller.TrackReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Hub:    srv.Hub,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create Track controller")
		os.Exit(1)
	}

	if err := controller.NewStepReconciler(
		mgr.GetClient(), mgr.GetScheme(), srv.Hub,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create Step controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting provider-webaudio")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
