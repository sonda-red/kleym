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

// IdentityBoundary declares the platform-controlled Pod label that separates
// one workload variant from other identities in the same namespace and service account.
type IdentityBoundary struct {
	// labelKey is the reserved Kubernetes label key used as the structural exclusivity boundary.
	// +required
	// +kubebuilder:validation:MinLength=26
	// +kubebuilder:validation:MaxLength=88
	// +kubebuilder:validation:Pattern=`^identity\.kleym\.sonda\.red/[A-Za-z0-9]([A-Za-z0-9_.-]*[A-Za-z0-9])?$`
	LabelKey string `json:"labelKey"`

	// labelValue identifies this binding's workload variant.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([A-Za-z0-9_.-]*[A-Za-z0-9])?$`
	LabelValue string `json:"labelValue"`
}

// -------------------------------------------------------------------------
// Spec
// -------------------------------------------------------------------------

// InferenceIdentityBindingSpec defines the desired state of InferenceIdentityBinding.
type InferenceIdentityBindingSpec struct {

	// poolRef specifies the required InferencePool workload anchor in the same namespace.
	// +required
	PoolRef InferencePoolTargetRef `json:"poolRef"`

	// serviceAccountName is the DNS-1123 subdomain Kubernetes service account required in every rendered identity selector set.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	ServiceAccountName string `json:"serviceAccountName"`

	// identityBoundary declares the required platform-controlled label boundary for this workload variant.
	// +required
	IdentityBoundary IdentityBoundary `json:"identityBoundary"`
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

// IdentityBoundaryStatus records the validated boundary used for identity rendering.
type IdentityBoundaryStatus struct {
	// labelKey is the validated Kubernetes label key.
	// +required
	LabelKey string `json:"labelKey"`

	// labelValue is the validated Kubernetes label value.
	// +required
	LabelValue string `json:"labelValue"`
}

// BindingReference identifies a namespaced peer binding.
type BindingReference struct {
	// namespace is the peer binding namespace.
	// +required
	Namespace string `json:"namespace"`

	// name is the peer binding name.
	// +required
	Name string `json:"name"`
}

// IdentityBoundaryConflictStatus describes one precise conflict with a peer binding.
type IdentityBoundaryConflictStatus struct {
	// bindingRef identifies the peer binding.
	// +required
	BindingRef BindingReference `json:"bindingRef"`

	// cause identifies the structural exclusivity failure.
	// +required
	// +kubebuilder:validation:Enum=BoundaryValueReuse;BoundaryKeyMismatch;DuplicateSPIFFEID
	Cause string `json:"cause"`

	// spiffeID is the peer's rendered SPIFFE ID.
	// +required
	SpiffeID string `json:"spiffeID"`

	// labelKey is the peer boundary key when it was resolved.
	// +optional
	LabelKey string `json:"labelKey,omitempty"`

	// value is the peer boundary value when it was resolved.
	// +optional
	Value string `json:"value,omitempty"`
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
	//   - "Conflict"
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

	// identityBoundary records the validated boundary used for the latest evaluation.
	// +optional
	IdentityBoundary *IdentityBoundaryStatus `json:"identityBoundary,omitempty"`

	// conflicts lists deterministic peer diagnoses when Conflict=True.
	// +optional
	Conflicts []IdentityBoundaryConflictStatus `json:"conflicts,omitempty"`

	// computedSpiffeIDs lists the SPIFFE IDs produced from the binding references.
	// +optional
	ComputedSpiffeIDs []ComputedSpiffeIDStatus `json:"computedSpiffeIDs,omitempty"`

	// renderedSelectors shows the final workload selectors for each rendered identity.
	// +optional
	RenderedSelectors []RenderedSelectorStatus `json:"renderedSelectors,omitempty"`

	// pendingClusterSPIFFEIDName is the deterministic name reserved before creating
	// a managed ClusterSPIFFEID. It remains set until creation is confirmed or
	// absence is confirmed during cleanup.
	// +optional
	PendingClusterSPIFFEIDName string `json:"pendingClusterSPIFFEIDName,omitempty"`

	// ownedClusterSPIFFEIDName is the deterministic name of the managed ClusterSPIFFEID
	// whose creation has been confirmed for this binding.
	// +optional
	OwnedClusterSPIFFEIDName string `json:"ownedClusterSPIFFEIDName,omitempty"`

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
// +kubebuilder:printcolumn:name="BOUNDARY",type=string,JSONPath=`.status.identityBoundary.labelValue`
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
