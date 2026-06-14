package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webaudiov1alpha1 "github.com/example/provider-webaudio/apis/webaudio/v1alpha1"
	"github.com/example/provider-webaudio/internal/server"
)

// TrackReconciler watches Track resources and ensures the Web Audio engine
// has the corresponding audio node created and configured.
type TrackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Hub    *server.Hub
}

// +kubebuilder:rbac:groups=webaudio.example.com,resources=tracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webaudio.example.com,resources=tracks/status,verbs=get;update;patch

func (r *TrackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var track webaudiov1alpha1.Track
	if err := r.Get(ctx, req.NamespacedName, &track); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling track",
		"name", track.Name,
		"instrument", track.Spec.Instrument,
		"volume", track.Spec.Volume,
	)

	nodeExists := track.Status.NodeID != ""

	if !nodeExists {
		log.Info("creating audio node", "track", track.Name, "instrument", track.Spec.Instrument)
		track.Status.NodeID = fmt.Sprintf("node-%s", track.Name)
		track.Status.Ready = true
	}

	if err := r.Status().Update(ctx, &track); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating track status: %w", err)
	}

	return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
}

func (r *TrackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webaudiov1alpha1.Track{}).
		Owns(&webaudiov1alpha1.Step{}).
		Complete(r)
}
