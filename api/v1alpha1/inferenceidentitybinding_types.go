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

// SelectorSource describes how workload selectors are derived.
type SelectorSource string

const (
	// SelectorSourceDerivedFromPool indicates that the selector is derived from the InferencePool's selector.
	SelectorSourceDerivedFromPool SelectorSource = "DerivedFromPool"
)

// ContainerDiscriminatorType defines which container attribute is used to discriminate identities.
type ContainerDiscriminatorType string

const (
	// ContainerDiscriminatorTypeName uses the container name as a discriminator.
	ContainerDiscriminatorTypeName ContainerDiscriminatorType = "ContainerName"
	// ContainerDiscriminatorTypeImage uses the container image as a discriminator.
	ContainerDiscriminatorTypeImage ContainerDiscriminatorType = "ContainerImage"
)

// -------------------------------------------------------------------------
// Spec helpers
// -------------------------------------------------------------------------

// InferenceObjectiveTargetRef is a reference to an InferenceObjective in the same namespace.
type InferenceObjectiveTargetRef struct {
	// name of the InferenceObjective resource to bind to.
	// +required
	Name string `json:"name"`
}

// ContainerDiscriminator specifies how to discriminate between containers
// when generating identities in PerObjective mode.
type ContainerDiscriminator struct {
	// type specifies the kind of discriminator to use (ContainerName or ContainerImage).
	// +required
	// +kubebuilder:validation:Enum=ContainerName;ContainerImage
	Type ContainerDiscriminatorType `json:"type"`

	// value is the container name or image to match, depending on type.
	// +required
	Value string `json:"value"`
}

// -------------------------------------------------------------------------
// Spec
// -------------------------------------------------------------------------

// InferenceIdentityBindingSpec defines the desired state of InferenceIdentityBinding.
// +kubebuilder:validation:XValidation:rule="!has(self.mode) || self.mode != 'PoolOnly' || !has(self.containerDiscriminator)",message="containerDiscriminator must be empty when mode is PoolOnly"
// +kubebuilder:validation:XValidation:rule="has(self.containerDiscriminator) || (has(self.mode) && self.mode == 'PoolOnly')",message="containerDiscriminator is required when mode is PerObjective (including default mode)"
type InferenceIdentityBindingSpec struct {

	// targetRef specifies the reference to the InferenceObjective in the same namespace.
	// +required
	TargetRef InferenceObjectiveTargetRef `json:"targetRef"`

	// spiffeIDTemplate optionally overrides the SPIFFE ID template for this binding.
	// If not specified, the default template from the controller configuration will be used.
	// +optional
	SpiffeIDTemplate *string `json:"spiffeIDTemplate,omitempty"`

	// selectorSource defines how the workload selectors are derived.
	// +kubebuilder:validation:Enum=DerivedFromPool
	// +required
	SelectorSource SelectorSource `json:"selectorSource"`

	// workloadSelectorTemplates are Go-template strings that produce SPIFFE workload
	// selectors for this binding. At least one template is required.
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +required
	WorkloadSelectorTemplates []string `json:"workloadSelectorTemplates"`

	// mode selects the identity granularity for this binding.
	// +kubebuilder:validation:Enum=PoolOnly;PerObjective
	// +kubebuilder:default=PerObjective
	// +optional
	Mode InferenceIdentityBindingMode `json:"mode,omitempty"`

	// containerDiscriminator specifies how to discriminate between containers when
	// generating identities in PerObjective mode.
	// Required when mode is PerObjective; must be empty when mode is PoolOnly.
	// +optional
	ContainerDiscriminator *ContainerDiscriminator `json:"containerDiscriminator,omitempty"`
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

// RenderedSelectorStatus describes the final selectors for one rendered identity,
// after processing WorkloadSelectorTemplates and SelectorSource.
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
	//   - "Available":   the resource is fully functional
	//   - "Progressing": the resource is being created or updated
	//   - "Degraded":    the resource failed to reach or maintain its desired state
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// computedSpiffeIDs lists the SPIFFE IDs produced from the SpiffeIDTemplate and TargetRef.
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

func init() {
	SchemeBuilder.Register(&InferenceIdentityBinding{}, &InferenceIdentityBindingList{})
}
