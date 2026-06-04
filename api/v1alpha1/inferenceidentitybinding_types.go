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
// Enums & Constants
// -------------------------------------------------------------------------

// InferenceIdentityBindingMode selects identity granularity.
// +kubebuilder:validation:Enum=PoolOnly;PerObjective
type InferenceIdentityBindingMode string

const (
	// InferenceIdentityBindingModePoolOnly indicates that a single identity is used for all workloads in the pool.
	InferenceIdentityBindingModePoolOnly InferenceIdentityBindingMode = "PoolOnly"
	// InferenceIdentityBindingModePerObjective indicates that a unique identity is generated for each InferenceObjective.
	InferenceIdentityBindingModePerObjective InferenceIdentityBindingMode = "PerObjective"
)

// -------------------------------------------------------------------------
// Spec helpers
// -------------------------------------------------------------------------

// InferencePoolTargetRef anchors a binding to the serving pool that supplies
// selector provenance. GAIE objectives are optional at request time, but pool
// selection is the stable workload boundary; see docs/spec/operator.md.
type InferencePoolTargetRef struct {
	// name of the InferencePool resource to bind to.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// group optionally constrains pool resolution to one GAIE API group.
	// Omit this when the cluster only serves one supported InferencePool group.
	// +optional
	// +kubebuilder:validation:Enum=inference.networking.k8s.io;inference.networking.x-k8s.io
	Group string `json:"group,omitempty"`
}

// InferenceObjectiveTargetRef is an optional reference to an InferenceObjective
// in the same namespace. It exists for PerObjective identity subjects while
// keeping PoolOnly identity independent of GAIE's alpha objective API.
type InferenceObjectiveTargetRef struct {
	// name of the InferenceObjective resource to bind to.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// group optionally constrains objective resolution to one GAIE API group.
	// +optional
	// +kubebuilder:validation:Enum=inference.networking.k8s.io;inference.networking.x-k8s.io
	Group string `json:"group,omitempty"`
}

// -------------------------------------------------------------------------
// Spec
// -------------------------------------------------------------------------

// InferenceIdentityBindingSpec defines the desired state of InferenceIdentityBinding.
// +kubebuilder:validation:XValidation:rule="!has(self.mode) || self.mode != 'PoolOnly' || !has(self.containerName)",message="containerName must be empty when mode is PoolOnly"
// +kubebuilder:validation:XValidation:rule="has(self.containerName) || (has(self.mode) && self.mode == 'PoolOnly')",message="containerName is required when mode is PerObjective (including default mode)"
// +kubebuilder:validation:XValidation:rule="has(self.objectiveRef) || (has(self.mode) && self.mode == 'PoolOnly')",message="objectiveRef is required when mode is PerObjective (including default mode)"
type InferenceIdentityBindingSpec struct {

	// poolRef specifies the required InferencePool workload anchor in the same namespace.
	// +required
	PoolRef InferencePoolTargetRef `json:"poolRef"`

	// objectiveRef optionally specifies an InferenceObjective in the same namespace.
	// It is required for PerObjective mode and, when present, must point at poolRef.
	// +optional
	ObjectiveRef *InferenceObjectiveTargetRef `json:"objectiveRef,omitempty"`

	// serviceAccountName is the Kubernetes service account required in every rendered identity selector set.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ServiceAccountName string `json:"serviceAccountName"`

	// mode selects the identity granularity for this binding.
	// +kubebuilder:default=PerObjective
	// +optional
	Mode InferenceIdentityBindingMode `json:"mode,omitempty"`

	// containerName specifies the serving container when generating identities in PerObjective mode.
	// Required when mode is PerObjective; must be empty when mode is PoolOnly.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	ContainerName string `json:"containerName,omitempty"`
}

// -------------------------------------------------------------------------
// Status helpers
// -------------------------------------------------------------------------

// ComputedSpiffeIDStatus holds a single computed SPIFFE ID and the mode that produced it.
type ComputedSpiffeIDStatus struct {
	// mode indicates the identity-generation mode (PoolOnly or PerObjective).
	// +optional
	Mode InferenceIdentityBindingMode `json:"mode,omitempty"`

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

// -------------------------------------------------------------------------
// Status
// -------------------------------------------------------------------------

// InferenceIdentityBindingStatus defines the observed state of InferenceIdentityBinding.
type InferenceIdentityBindingStatus struct {

	// conditions represent the latest available observations of the binding's state.
	//
	// Known condition types:
	//   - "Ready"
	//   - "Conflict"
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
}

// -------------------------------------------------------------------------
// Root types
// -------------------------------------------------------------------------

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
