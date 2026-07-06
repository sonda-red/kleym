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
package identity

import (
	"k8s.io/apimachinery/pkg/types"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	// DefaultTrustDomain preserves the original single-install rendering behavior for callers
	// that do not have operator deployment configuration available.
	DefaultTrustDomain = "kleym.sonda.red"

	// ConditionTypeUnsafeSelector matches the controller status condition for unsafe selector rendering.
	ConditionTypeUnsafeSelector = "UnsafeSelector"
	// ConditionTypeRenderFailure matches the controller status condition for render failures.
	ConditionTypeRenderFailure = "RenderFailure"

	// ReasonMissingTrustDomain reports missing operator identity configuration.
	ReasonMissingTrustDomain = "MissingTrustDomain"
	// ReasonInvalidServiceAccountName reports an invalid binding service account boundary.
	ReasonInvalidServiceAccountName = "InvalidServiceAccountName"
	// ReasonUnsafeSelector reports a rendered selector set that violates safety invariants.
	ReasonUnsafeSelector = "UnsafeSelector"
	// ReasonInvalidSPIFFEID reports a computed SPIFFE ID that fails validation.
	ReasonInvalidSPIFFEID = "InvalidSPIFFEID"
	// ReasonInvalidPoolSelector reports a GAIE pool selector that cannot be rendered safely.
	ReasonInvalidPoolSelector = "InvalidPoolSelector"
)

// StateError carries condition metadata for shared identity computation errors.
type StateError struct {
	ConditionType string
	Reason        string
	Message       string
}

func (e *StateError) Error() string {
	return e.Message
}

func newStateError(conditionType, reason, message string) *StateError {
	return &StateError{
		ConditionType: conditionType,
		Reason:        reason,
		Message:       message,
	}
}

// PlanInput carries already-resolved identity planning inputs.
type PlanInput struct {
	Binding              *kleymv1alpha1.InferenceIdentityBinding
	TrustDomain          string
	PoolName             string
	PodSelector          map[string]any
	PoolDerivedSelectors []string
}

// Plan is the pure desired identity state shared by the controller and CLI.
type Plan struct {
	SpiffeID    string
	Selectors   []string
	PodSelector map[string]any
	PoolRef     string
}

// RenderedIdentity is kept as a compatibility alias for callers during the package split.
type RenderedIdentity = Plan

type renderTemplateData struct {
	Namespace   string
	BindingName string
	PoolName    string
}

// NamespacedBindingKey returns the canonical namespace/name key used in logs and messages.
func NamespacedBindingKey(namespace, name string) string {
	return types.NamespacedName{Namespace: namespace, Name: name}.String()
}
