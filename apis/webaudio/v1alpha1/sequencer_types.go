package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Minimum=20
// +kubebuilder:validation:Maximum=300
type BPMValue int

// SequencerSpec defines the desired state of a Sequencer.
type SequencerSpec struct {
	// +kubebuilder:validation:Minimum=20
	// +kubebuilder:validation:Maximum=300
	BPM int `json:"bpm"`
	// +kubebuilder:validation:Enum=4;8;16
	Steps   int  `json:"steps"`
	Running bool `json:"running"`
}

// SequencerStatus is the observed state reported by the reconciler.
type SequencerStatus struct {
	Ready           bool         `json:"ready,omitempty"`
	ObservedBPM     int          `json:"observedBPM,omitempty"`
	ObservedRunning bool         `json:"observedRunning,omitempty"`
	LastSyncTime    string `json:"lastSyncTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=webaudio
// +kubebuilder:printcolumn:name="BPM",type=integer,JSONPath=`.spec.bpm`
// +kubebuilder:printcolumn:name="RUNNING",type=boolean,JSONPath=`.spec.running`
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`

// Sequencer is the top-level resource. It owns Tracks, which own Steps.
type Sequencer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SequencerSpec   `json:"spec"`
	Status            SequencerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SequencerList contains a list of Sequencer resources.
type SequencerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sequencer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sequencer{}, &SequencerList{})
}
