package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StepSpec defines the desired state of a single step in the grid.
type StepSpec struct {
	// +kubebuilder:validation:Required
	TrackRef string `json:"trackRef"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=15
	Index    int     `json:"index"`
	Active   bool    `json:"active"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default=1.0
	Velocity float64 `json:"velocity"`
}

// StepStatus is the observed state from the Web Audio engine.
type StepStatus struct {
	ObservedActive     bool         `json:"observedActive,omitempty"`
	DriftDetected      bool         `json:"driftDetected,omitempty"`
	LastCorrectionTime string `json:"lastCorrectionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=webaudio
// +kubebuilder:printcolumn:name="TRACK",type=string,JSONPath=`.spec.trackRef`
// +kubebuilder:printcolumn:name="INDEX",type=integer,JSONPath=`.spec.index`
// +kubebuilder:printcolumn:name="ACTIVE",type=boolean,JSONPath=`.spec.active`
// +kubebuilder:printcolumn:name="DRIFT",type=boolean,JSONPath=`.status.driftDetected`

// Step is a single cell in the sequencer grid.
type Step struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              StepSpec   `json:"spec"`
	Status            StepStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StepList contains a list of Step resources.
type StepList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Step `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Step{}, &StepList{})
}
