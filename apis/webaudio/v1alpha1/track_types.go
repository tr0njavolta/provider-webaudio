package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=sine;square;sawtooth;triangle;noise
type Waveform string

const (
	WaveformSine     Waveform = "sine"
	WaveformSquare   Waveform = "square"
	WaveformSawtooth Waveform = "sawtooth"
	WaveformTriangle Waveform = "triangle"
	WaveformNoise    Waveform = "noise"
)

// TrackSpec defines the desired state of a Track.
type TrackSpec struct {
	// +kubebuilder:validation:Required
	SequencerRef string `json:"sequencerRef"`
	// +kubebuilder:validation:Enum=synth
	Instrument string `json:"instrument"`
	// +kubebuilder:optional
	Waveform Waveform `json:"waveform,omitempty"`
	// +kubebuilder:validation:Minimum=20
	// +kubebuilder:validation:Maximum=20000
	// +kubebuilder:optional
	Frequency float64 `json:"frequency,omitempty"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted,omitempty"`
}

// TrackStatus reflects what the Web Audio engine has created.
type TrackStatus struct {
	NodeID string `json:"nodeID,omitempty"`
	Ready  bool   `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=webaudio
// +kubebuilder:printcolumn:name="SEQUENCER",type=string,JSONPath=`.spec.sequencerRef`
// +kubebuilder:printcolumn:name="INSTRUMENT",type=string,JSONPath=`.spec.instrument`
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=`.status.ready`

// Track is one row in the sequencer grid.
type Track struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TrackSpec   `json:"spec"`
	Status            TrackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TrackList contains a list of Track resources.
type TrackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Track `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Track{}, &TrackList{})
}
