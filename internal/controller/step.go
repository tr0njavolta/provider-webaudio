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

// StepReconciler is where drift detection is most visible.
//
// The flow that teaches the core Crossplane concept:
//
//  1. User clicks a step in the browser UI
//  2. Browser sends: PATCH /api/steps/{name} { active: true }
//  3. The HTTP handler writes to hub.Patches
//  4. StepReconciler.HandlePatch() receives the patch and writes
//     status.observedActive = true (but does NOT touch spec.active)
//  5. Reconcile() fires, sees spec.active != status.observedActive
//  6. Reconciler corrects: sets status.observedActive = spec.active,
//     sets status.driftDetected = false, pushes corrected state to browser
//  7. Browser receives the corrected state and reverts the grid cell
//
// The user watches Kubernetes enforce desired state in real time.
type StepReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Hub    *server.Hub
}

func NewStepReconciler(c client.Client, scheme *runtime.Scheme, hub *server.Hub) *StepReconciler {
	r := &StepReconciler{
		Client: c,
		Scheme: scheme,
		Hub:    hub,
	}
	// Start the goroutine that drains browser patch events.
	go r.watchPatches()
	return r
}

// watchPatches drains the hub's Patches channel and applies each one.
// This runs in a goroutine for the lifetime of the controller.
func (r *StepReconciler) watchPatches() {
	for patch := range r.Hub.Patches {
		ctx := context.Background()
		log := log.FromContext(ctx)

		var step webaudiov1alpha1.Step
		if err := r.Get(ctx, client.ObjectKey{Name: patch.StepName}, &step); err != nil {
			log.Error(err, "step not found for patch", "step", patch.StepName)
			continue
		}

		// The browser tried to change active state. Record this as observed
		// state divergence — do NOT change spec. The reconciler will correct it.
		step.Status.ObservedActive = patch.Active
		step.Status.DriftDetected = patch.Active != step.Spec.Active

		if step.Status.DriftDetected {
			log.Info("drift recorded from browser patch",
				"step", step.Name,
				"specActive", step.Spec.Active,
				"observedActive", patch.Active,
			)
		}

		if err := r.Status().Update(ctx, &step); err != nil {
			log.Error(err, "failed to record drift", "step", step.Name)
		}
		// The status update triggers a reconcile, which will correct the drift.
	}
}

// +kubebuilder:rbac:groups=webaudio.example.com,resources=steps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webaudio.example.com,resources=steps/status,verbs=get;update;patch

func (r *StepReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// ── Observe ──────────────────────────────────────────────────────────────
	var step webaudiov1alpha1.Step
	if err := r.Get(ctx, req.NamespacedName, &step); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ── Compare ──────────────────────────────────────────────────────────────
	// The key drift check: is what the engine is doing (observedActive)
	// different from what we declared (spec.active)?
	drift := step.Status.ObservedActive != step.Spec.Active

	if drift {
		log.Info("correcting step drift",
			"step", step.Name,
			"desired", step.Spec.Active,
			"observed", step.Status.ObservedActive,
		)
	}

	// ── Correct ──────────────────────────────────────────────────────────────
	// Drive observed state to match desired state.
	// In a real cloud provider this is where you'd call the cloud API.
	// Here it's updating our status to reflect the correction, then
	// the Sequencer reconciler will pick it up and push to the browser.
	now := time.Now()
	step.Status.ObservedActive = step.Spec.Active // enforce desired state
	step.Status.DriftDetected = false

	if drift {
		step.Status.LastCorrectionTime = now.UTC().Format(time.RFC3339)
	}

	// ── Report ───────────────────────────────────────────────────────────────
	if err := r.Status().Update(ctx, &step); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating step status: %w", err)
	}

	// The Sequencer reconciler will pick up the change and broadcast
	// corrected state to the browser on its next cycle.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *StepReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index Steps by trackRef so Track reconciler can list them efficiently.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&webaudiov1alpha1.Step{},
		"spec.trackRef",
		func(obj client.Object) []string {
			step := obj.(*webaudiov1alpha1.Step)
			return []string{step.Spec.TrackRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&webaudiov1alpha1.Step{}).
		Complete(r)
}
