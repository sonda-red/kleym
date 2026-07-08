/*
Copyright 2026 Kalin Daskalov.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -------------------------------------------------------------------------
// Spec helpers
// -------------------------------------------------------------------------

// InferencePoolTargetRef anchors a binding to the serving pool that supplies
// selector provenance. Pool selection is the stable workload boundary; see
// docs/spec/operator.md.
type InferencePoolTargetRef struct {
	// name of the InferencePool resource to bind to.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// group optionally constrains pool resolution to the supported GAIE API group.
	// Omit this to use the served supported InferencePool group.
	// +optional
	// +kubebuilder:validation:Enum=inference.networking.k8s.io
	Group string `json:"group,omitempty"`
}

// -------------------------------------------------------------------------
// Spec
// -------------------------------------------------------------------------

// InferenceIdentityBindingSpec defines the desired state of InferenceIdentityBinding.
type InferenceIdentityBindingSpec struct {

	// poolRef specifies the required InferencePool workload anchor in the same namespace.
	// +required
	PoolRef InferencePoolTargetRef `json:"poolRef"`

	// serviceAccountName is the Kubernetes service account required in every rendered identity selector set.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ServiceAccountName string `json:"serviceAccountName"`
}

// -------------------------------------------------------------------------
// Status helpers
// -------------------------------------------------------------------------

// ComputedSpiffeIDStatus holds a single computed SPIFFE ID.
type ComputedSpiffeIDStatus struct {
	// spiffeID is the computed SPIFFE ID for this binding.
	// +required
	SpiffeID string `json:"spiffeID"`
}

// RenderedSelectorStatus describes the final selectors for one rendered identity.
type RenderedSelectorStatus struct {
	// spiffeID is the computed SPIFFE ID for this rendered identity.
	// +required
	SpiffeID string `json:"spiffeID"`

	// selectors are the final set of workload selectors applied to this identity.
	// +listType=set
	// +optional
	Selectors []string `json:"selectors,omitempty"`
}

// RenderedClusterSPIFFEIDStatus describes the core managed ClusterSPIFFEID output.
type RenderedClusterSPIFFEIDStatus struct {
	// name is the deterministic managed ClusterSPIFFEID name rendered for this binding.
	// +required
	Name string `json:"name"`

	// spiffeID is the rendered SPIFFE ID written to the managed ClusterSPIFFEID.
	// +required
	SpiffeID string `json:"spiffeID"`

	// selectorFingerprint is the deterministic sha256 fingerprint of the canonical selector set.
	// +required
	SelectorFingerprint string `json:"selectorFingerprint"`

	// observedGeneration is the observed metadata.generation of the managed ClusterSPIFFEID.
	// It is omitted when Kubernetes has not reported a persisted generation for the managed resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// -------------------------------------------------------------------------
// Status
// -------------------------------------------------------------------------

// InferenceIdentityBindingStatus defines the observed state of InferenceIdentityBinding.
type InferenceIdentityBindingStatus struct {

	// conditions represent the latest available observations of the binding's state.
	//
	// Known condition types:
	//   - "Ready"
	//   - "InvalidRef"
	//   - "UnsafeSelector"
	//   - "RenderFailure"
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// trustDomain is the operator trust domain used when rendering the latest status.
	// +optional
	TrustDomain string `json:"trustDomain,omitempty"`

	// clusterSPIFFEIDClassName is the operator ClusterSPIFFEID class name used
	// when rendering the latest status. Empty means classless ClusterSPIFFEID output.
	// +optional
	ClusterSPIFFEIDClassName string `json:"clusterSPIFFEIDClassName,omitempty"`

	// computedSpiffeIDs lists the SPIFFE IDs produced from the binding references.
	// +optional
	ComputedSpiffeIDs []ComputedSpiffeIDStatus `json:"computedSpiffeIDs,omitempty"`

	// renderedSelectors shows the final workload selectors for each rendered identity.
	// +optional
	RenderedSelectors []RenderedSelectorStatus `json:"renderedSelectors,omitempty"`

	// renderedClusterSPIFFEID shows the core rendered managed ClusterSPIFFEID output.
	// +optional
	RenderedClusterSPIFFEID *RenderedClusterSPIFFEIDStatus `json:"renderedClusterSPIFFEID,omitempty"`
}

// -------------------------------------------------------------------------
// Root types
// -------------------------------------------------------------------------

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="POOL",type=string,JSONPath=`.spec.poolRef.name`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="REASON",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="SPIFFE ID",type=string,JSONPath=`.status.computedSpiffeIDs[0].spiffeID`

// InferenceIdentityBinding is the Schema for the inferenceidentitybindings API.
type InferenceIdentityBinding struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of InferenceIdentityBinding.
	// +required
	Spec InferenceIdentityBindingSpec `json:"spec"`

	// status defines the observed state of InferenceIdentityBinding.
	// +optional
	Status InferenceIdentityBindingStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// InferenceIdentityBindingList contains a list of InferenceIdentityBinding.
type InferenceIdentityBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InferenceIdentityBinding `json:"items"`
}
