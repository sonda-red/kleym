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
	"k8s.io/apimachinery/pkg/types"
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

// IdentityBoundary declares the platform-controlled workload variant that
// separates identities in the same namespace and service account.
type IdentityBoundary struct {
	// variant identifies this binding's workload variant and is rendered under
	// the operator-owned identity.kleym.sonda.red/variant Pod label key.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([A-Za-z0-9_.-]*[A-Za-z0-9])?$`
	Variant string `json:"variant"`
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

// PendingClusterSPIFFEIDStatus records a durable create claim before the
// corresponding ClusterSPIFFEID is submitted to the Kubernetes API.
type PendingClusterSPIFFEIDStatus struct {
	// name is the deterministic managed ClusterSPIFFEID name reserved by this claim.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// claimID is the controller-generated correlation token carried by the created object.
	// +required
	// +kubebuilder:validation:MinLength=1
	ClaimID string `json:"claimID"`
}

// OwnedClusterSPIFFEIDStatus records the exact managed ClusterSPIFFEID
// incarnation that this binding may update or delete.
type OwnedClusterSPIFFEIDStatus struct {
	// name is the deterministic name of the confirmed managed ClusterSPIFFEID.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// uid is the Kubernetes UID of the confirmed managed ClusterSPIFFEID incarnation.
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	UID types.UID `json:"uid"`
}

// VariantConflictStatus describes one precise conflict with a peer binding.
type VariantConflictStatus struct {
	// bindingName identifies the same-namespace peer binding.
	// +required
	BindingName string `json:"bindingName"`

	// cause identifies the variant exclusivity failure.
	// +required
	// +kubebuilder:validation:Enum=VariantReuse;DuplicateSPIFFEID
	Cause string `json:"cause"`

	// spiffeID is the peer's rendered SPIFFE ID.
	// +required
	SpiffeID string `json:"spiffeID"`

	// variant is the peer's validated workload variant.
	// +required
	Variant string `json:"variant"`
}

// -------------------------------------------------------------------------
// Status
// -------------------------------------------------------------------------

// InferenceIdentityBindingStatus defines the observed state of InferenceIdentityBinding.
// +kubebuilder:validation:XValidation:rule="!(has(self.pendingClusterSPIFFEID) && has(self.ownedClusterSPIFFEID))",message="pendingClusterSPIFFEID and ownedClusterSPIFFEID are mutually exclusive"
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

	// conflicts lists deterministic peer diagnoses when Conflict=True.
	// +optional
	Conflicts []VariantConflictStatus `json:"conflicts,omitempty"`

	// computedSpiffeIDs lists the SPIFFE IDs produced from the binding references.
	// +optional
	ComputedSpiffeIDs []ComputedSpiffeIDStatus `json:"computedSpiffeIDs,omitempty"`

	// renderedSelectors shows the final workload selectors for each rendered identity.
	// +optional
	RenderedSelectors []RenderedSelectorStatus `json:"renderedSelectors,omitempty"`

	// pendingClusterSPIFFEID records the name and unique claim ID persisted before
	// creating a managed ClusterSPIFFEID. A matching claim annotation correlates
	// create-success recovery with the exact object created for this binding.
	// +optional
	PendingClusterSPIFFEID *PendingClusterSPIFFEIDStatus `json:"pendingClusterSPIFFEID,omitempty"`

	// ownedClusterSPIFFEID records the name and Kubernetes UID of the exact managed
	// ClusterSPIFFEID incarnation whose ownership has been confirmed for this binding.
	// +optional
	OwnedClusterSPIFFEID *OwnedClusterSPIFFEIDStatus `json:"ownedClusterSPIFFEID,omitempty"`

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
// +kubebuilder:printcolumn:name="BOUNDARY",type=string,JSONPath=`.spec.identityBoundary.variant`
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
