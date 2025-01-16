package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WatchSelector defines the resources/namespace to watch
type WatchSelector struct {
	// Namespace is the namespace to watch resource in
	Namespace string `json:"namespace"`

	// Kinds is the list of kind to watch and audit. e.g (Deployment, Services)
	Kinds []string `json:"kinds,omitempty"`
}

// WatchSpec defines the desired state of Watch.
type WatchSpec struct {
	Selectors []WatchSelector `json:"selectors"`
}

// WatchStatus defines the observed state of Watch.
type WatchStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Watch is the Schema for the watches API.
type Watch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WatchSpec   `json:"spec,omitempty"`
	Status WatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WatchList contains a list of Watch.
type WatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Watch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Watch{}, &WatchList{})
}
