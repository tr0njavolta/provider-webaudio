package controller

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"

	webaudiov1alpha1 "github.com/example/provider-webaudio/apis/webaudio/v1alpha1"
	"github.com/example/provider-webaudio/internal/server"
)

type SequencerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Hub    *server.Hub
	engine map[string]SequencerObserved
}

type SequencerObserved struct {
	BPM     int
	Running bool
}

func NewSequencerReconciler(c client.Client, scheme *runtime.Scheme, hub *server.Hub) *SequencerReconciler {
	return &SequencerReconciler{
		Client: c,
		Scheme: scheme,
		Hub:    hub,
		engine: make(map[string]SequencerObserved),
	}
}

// +kubebuilder:rbac:groups=webaudio.example.com,resources=sequencers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webaudio.example.com,resources=sequencers/status,verbs=get;update;patch

func (r *SequencerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var seq webaudiov1alpha1.Sequencer
	if err := r.Get(ctx, req.NamespacedName, &seq); err != nil {
		delete(r.engine, req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling sequencer", "name", seq.Name, "bpm", seq.Spec.BPM, "running", seq.Spec.Running)

	state, err := r.buildStatePayload(ctx, &seq)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building state payload: %w", err)
	}

	select {
	case r.Hub.Broadcast <- state:
		r.engine[seq.Name] = SequencerObserved{BPM: seq.Spec.BPM, Running: seq.Spec.Running}
	default:
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	seq.Status.Ready = true
	seq.Status.ObservedBPM = seq.Spec.BPM
	seq.Status.ObservedRunning = seq.Spec.Running
	seq.Status.LastSyncTime = time.Now().UTC().Format(time.RFC3339)

	if err := r.Status().Update(ctx, &seq); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *SequencerReconciler) buildStatePayload(ctx context.Context, seq *webaudiov1alpha1.Sequencer) (server.StatePayload, error) {
	var allTracks webaudiov1alpha1.TrackList
	if err := r.List(ctx, &allTracks, client.InNamespace(seq.Namespace)); err != nil {
		return server.StatePayload{}, fmt.Errorf("listing tracks: %w", err)
	}

	var tracks []server.TrackState
	for _, track := range allTracks.Items {
		if track.Spec.SequencerRef != seq.Name {
			continue
		}

		var allSteps webaudiov1alpha1.StepList
		if err := r.List(ctx, &allSteps, client.InNamespace(track.Namespace)); err != nil {
			return server.StatePayload{}, fmt.Errorf("listing steps: %w", err)
		}

		var steps []server.StepState
		for _, step := range allSteps.Items {
			if step.Spec.TrackRef != track.Name {
				continue
			}
			steps = append(steps, server.StepState{
				Name:           step.Name,
				Index:          step.Spec.Index,
				Active:         step.Spec.Active,
				ObservedActive: step.Status.ObservedActive,
				DriftDetected:  step.Status.DriftDetected,
				Velocity:       step.Spec.Velocity,
			})
		}

		tracks = append(tracks, server.TrackState{
			Name:       track.Name,
			Instrument: track.Spec.Instrument,
			Waveform:   string(track.Spec.Waveform),
			Frequency:  track.Spec.Frequency,
			Volume:     track.Spec.Volume,
			Muted:      track.Spec.Muted,
			Steps:      steps,
		})
	}

	return server.StatePayload{
		Sequencers: []server.SequencerState{{
			Name:    seq.Name,
			BPM:     seq.Spec.BPM,
			Running: seq.Spec.Running,
			Tracks:  tracks,
		}},
	}, nil
}

func (r *SequencerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webaudiov1alpha1.Sequencer{}).
		Owns(&webaudiov1alpha1.Track{}).
		Complete(r)
}
