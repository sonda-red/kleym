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

type SelectorSource string

const (
	// SelectorSourceDerivedFromPool indicates that the selector is derived from the InferencePool's selector.
	SelectorSourceDerivedFromPool SelectorSource = "DerivedFromPool"
)

// InferenceIdentityBindingMode selects identity granularity.
// +kubebuilder:validation:Enum=PoolOnly;PerObjective
type InferenceIdentityBindingMode string

const (
	// InferenceIdentityBindingModePoolOnly indicates that a single identity is used for all workloads in the pool.
	InferenceIdentityBindingModePoolOnly = "PoolOnly"
	// InferenceIdentityBindingModePerObjective indicates that a unique identity is generated for each InferenceObjective.
	InferenceIdentityBindingModePerObjective = "PerObjective"
)

type ContainerDiscriminatorType string

const (
	// ContainerDiscriminatorTypeName indicates that the container name is used as a discriminator for identity generation.
	ContainerDiscriminatorTypeName ContainerDiscriminatorType = "ContainerName"
	// ContainerDiscriminatorTypeImage indicates that the container image is used as a discriminator for identity generation.
	ContainerDiscriminatorTypeImage ContainerDiscriminatorType = "ContainerImage"
)

type ContainerDiscriminator struct {
	// Type specifies the type of discriminator to use (e.g., ContainerName, ContainerImage).
	// +required
	Type ContainerDiscriminatorType `json:"type"`

	// Value is the container name or image to use as a discriminator, depending on the Type.
	// +required
	Value string `json:"value"`
}

type ComputedSpiffeIDStatus struct {
	// mode indicates the mode of SPIFFE ID generation (e.g., PoolOnly, PerObjective).
	// +optional
	Mode InferenceIdentityBindingMode `json:"mode,omitempty"`

	// spiffeID is the computed SPIFFE ID for this binding
	// +required
	SpiffeID string `json:"spiffeID"`
}

// RenderedSelectorStatus describes final selectors for one rendered identity, after processing the WorkloadSelectorTemplates and SelectorSource.
type RenderedSelectorStatus struct {
	// spiffeID is the computed SPIFFE ID for this binding
	// +required
	SpiffeID string `json:"spiffeID"`

	// selectors are the final set of selectors that will be applied to workloads matching this binding.
	// +listType=set
	// +optional
	Selectors []string `json:"selectors,omitempty"`
}

type InferenceObjectiveTargetRef struct {
	// Name of the InferenceObjective resource to bind to.
	// +required
	Name string `json:"name"`
}

// InferenceIdentityBindingSpec defines the desired state of InferenceIdentityBinding
type InferenceIdentityBindingSpec struct {
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// TargetRef specifies the reference to the InferenceObjective in the same namespace
	// +required
	TargetRef InferenceObjectiveTargetRef `json:"targetRef"`

	// spiffeIDTemplate optionally overrides the SPIFFE ID template for this binding. If not specified, the default template from the controller configuration will be used.
	// +optional
	SpiffeIDTemplate *string `json:"spiffeIDTemplate,omitempty"`

	// selectorSource defines how the workload selectors are derived.
	// +kubebuilder:validation:Enum=DerivedFromPool
	// +required
	SelectorSource SelectorSource `json:"selectorSource"`

	// workloadSelectorTemplates are required safety templates that define how to derive workload selectors for this binding.
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +required
	WorkloadSelectorTemplates []string `json:"workloadSelectorTemplates"`

	// mode selects the identity granularity for this binding.
	// +kubebuilder:validation:Enum=PoolOnly;PerObjective
	// +kubebuilder:default=PerObjective
	// +optional
	Mode InferenceIdentityBindingMode `json:"mode,omitempty"`

	// containerDiscriminator specifies how to discriminate between containers when generating identities in PerObjective mode.
	// Required if mode is PerObjective, must be empty if mode is PoolOnly.
	// +optional
	ContainerDiscriminator *ContainerDiscriminator `json:"containerDiscriminator,omitempty"`
}

// InferenceIdentityBindingStatus defines the observed state of InferenceIdentityBinding.
type InferenceIdentityBindingStatus struct {
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the InferenceIdentityBinding resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// computedSpiffeID is the computed SPIFFE ID for this binding, based on the SpiffeIDTemplate and the TargetRef.
	// +optional
	ComputedSpiffeID []ComputedSpiffeIDStatus `json:"computedSpiffeID,omitempty"`

	// renderedSelectors shows final selectors applied to rendered identities, after processing the WorkloadSelectorTemplates and SelectorSource.
	// +optional
	RenderedSelectors []RenderedSelectorStatus `json:"renderedSelectors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InferenceIdentityBinding is the Schema for the inferenceidentitybindings API
type InferenceIdentityBinding struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of InferenceIdentityBinding
	// +required
	Spec InferenceIdentityBindingSpec `json:"spec"`

	// status defines the observed state of InferenceIdentityBinding
	// +optional
	Status InferenceIdentityBindingStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// InferenceIdentityBindingList contains a list of InferenceIdentityBinding
type InferenceIdentityBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InferenceIdentityBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferenceIdentityBinding{}, &InferenceIdentityBindingList{})
}
