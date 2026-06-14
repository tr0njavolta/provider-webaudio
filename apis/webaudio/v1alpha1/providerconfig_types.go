package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderConfigSpec defines configuration for the Web Audio provider.
type ProviderConfigSpec struct {
	// +kubebuilder:default=9090
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	ServerPort int `json:"serverPort,omitempty"`
	// +kubebuilder:default=44100
	// +kubebuilder:validation:Enum=22050;44100;48000;96000
	SampleRate int `json:"sampleRate,omitempty"`
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=60
	DriftCorrectionInterval int `json:"driftCorrectionInterval,omitempty"`
}

// ProviderConfigStatus reflects the observed state of the provider config.
type ProviderConfigStatus struct {
	Ready            bool `json:"ready,omitempty"`
	ConnectedClients int  `json:"connectedClients,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=webaudio
// +kubebuilder:printcolumn:name="PORT",type=integer,JSONPath=`.spec.serverPort`
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=`.status.ready`

// ProviderConfig configures the Web Audio provider.
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProviderConfigSpec   `json:"spec,omitempty"`
	Status            ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig resources.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
