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

// InferenceTrustBindingSpec defines trust and identity settings for inference workloads.
// This resource configures how kleym attaches SPIFFE identities, mTLS enforcement,
// and audit logging to existing inference deployments. It does NOT create or manage
// the inference workloads themselves—those are owned by llm-d, vLLM charts, or plain Deployments.
type InferenceTrustBindingSpec struct {
	// selector identifies which inference workloads this binding applies to.
	// Uses standard Kubernetes label selectors to match Pods or Deployments.
	// +required
	Selector metav1.LabelSelector `json:"selector"`

	// spiffeIDScope determines the granularity of SPIFFE identity assignment.
	// - "pod": Each pod receives a unique SPIFFE ID (recommended for audit granularity)
	// - "replicaSet": All pods in a ReplicaSet share one SPIFFE ID
	// - "deployment": All pods in a Deployment share one SPIFFE ID
	// +kubebuilder:validation:Enum=pod;replicaSet;deployment
	// +kubebuilder:default=pod
	// +optional
	SPIFFEIDScope string `json:"spiffeIdScope,omitempty"`

	// mtlsRequired enforces mutual TLS for all traffic to matched inference workloads.
	// When true, kleym ensures SPIRE-issued SVIDs are available and configures
	// mTLS enforcement (e.g., via sidecar injection or native SPIFFE support).
	// +kubebuilder:default=true
	// +optional
	MTLSRequired *bool `json:"mtlsRequired,omitempty"`

	// policyRef is an optional reference to an external policy resource (e.g., OPA ConfigMap)
	// that governs access control decisions for the matched workloads.
	// Policy integration is optional and pluggable—kleym does not implement policy engines.
	// +optional
	PolicyRef *PolicyReference `json:"policyRef,omitempty"`

	// attributionLog configures audit logging for identity attribution.
	// Logs include caller identity, workload SPIFFE ID, and request metadata.
	// +optional
	AttributionLog *AttributionLogConfig `json:"attributionLog,omitempty"`
}

// PolicyReference points to an external policy resource.
// kleym does not evaluate policies itself—it passes references to policy engines like OPA/Gatekeeper.
type PolicyReference struct {
	// kind is the type of policy resource (e.g., "ConfigMap", "OPAPolicy")
	// +kubebuilder:validation:MinLength=1
	// +required
	Kind string `json:"kind"`

	// name is the name of the policy resource
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// namespace is the namespace of the policy resource (defaults to binding namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AttributionLogConfig controls how kleym emits audit logs for identity attribution.
type AttributionLogConfig struct {
	// enabled turns attribution logging on or off
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// format specifies the log output format
	// - "json": Structured JSON logs (recommended for SIEM integration)
	// - "text": Human-readable text format
	// +kubebuilder:validation:Enum=json;text
	// +kubebuilder:default=json
	// +optional
	Format string `json:"format,omitempty"`

	// includeRequestMetadata adds request-level metadata (method, path, headers) to logs.
	// Note: This is deployment-level logging only. Per-request audit trails are out of MVP scope.
	// +kubebuilder:default=false
	// +optional
	IncludeRequestMetadata *bool `json:"includeRequestMetadata,omitempty"`
}

// InferenceTrustBindingStatus defines the observed state of InferenceTrustBinding.
type InferenceTrustBindingStatus struct {
	// matchedWorkloads is the count of inference workloads currently matched by this profile
	// +optional
	MatchedWorkloads int32 `json:"matchedWorkloads,omitempty"`

	// identitiesIssued is the count of SPIFFE identities issued for matched workloads
	// +optional
	IdentitiesIssued int32 `json:"identitiesIssued,omitempty"`

	// mtlsEnforced indicates whether mTLS is currently enforced for all matched workloads
	// +optional
	MTLSEnforced bool `json:"mtlsEnforced,omitempty"`

	// conditions represent the current state of the InferenceTrustBinding resource.
	// Condition types:
	// - "Ready": Binding is active and identities are being issued
	// - "SPIREConnected": SPIRE integration is healthy
	// - "PolicyResolved": Referenced policy (if any) is available
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InferenceTrustBinding is the Schema for the inferencetrustbindings API
type InferenceTrustBinding struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of InferenceTrustBinding
	// +required
	Spec InferenceTrustBindingSpec `json:"spec"`

	// status defines the observed state of InferenceTrustBinding
	// +optional
	Status InferenceTrustBindingStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// InferenceTrustBindingList contains a list of InferenceTrustBinding
type InferenceTrustBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InferenceTrustBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferenceTrustBinding{}, &InferenceTrustBindingList{})
}
