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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	defaultNameValue   = "kleym"
	defaultTrustDomain = "kleym.sonda.red"

	// ManagedByLabelKey identifies ClusterSPIFFEID resources owned by kleym.
	ManagedByLabelKey = "kleym.sonda.red/managed-by"
	// ManagedByLabelValue is the stable managed-by label value for kleym resources.
	ManagedByLabelValue = defaultNameValue
	// BindingNameLabelKey records the source InferenceIdentityBinding name.
	BindingNameLabelKey = "kleym.sonda.red/binding-name"
	// BindingNamespaceLabelKey records the source InferenceIdentityBinding namespace.
	BindingNamespaceLabelKey = "kleym.sonda.red/binding-namespace"

	// ConditionTypeInvalidRef matches the controller status condition for invalid input references.
	ConditionTypeInvalidRef = "InvalidRef"
	// ConditionTypeUnsafeSelector matches the controller status condition for unsafe selector rendering.
	ConditionTypeUnsafeSelector = "UnsafeSelector"
	// ConditionTypeRenderFailure matches the controller status condition for render failures.
	ConditionTypeRenderFailure = "RenderFailure"
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

// PoolRef is the normalized namespaced target for a GAIE InferencePool.
type PoolRef struct {
	Name      string
	Group     string
	Namespace string
}

// ObjectiveRef is the normalized namespaced target for a GAIE InferenceObjective.
type ObjectiveRef struct {
	Name      string
	Group     string
	Namespace string
}

// RenderedIdentity is the pure desired identity state shared by the controller and CLI.
type RenderedIdentity struct {
	Name         string
	Mode         kleymv1alpha1.InferenceIdentityBindingMode
	SpiffeID     string
	Selectors    []string
	PodSelector  map[string]any
	ObjectiveRef string
	PoolRef      string
	Hint         string
	Fallback     bool
}

type renderTemplateData struct {
	Namespace     string
	BindingName   string
	ObjectiveName string
	PoolName      string
	Mode          string
}

// NamespacedBindingKey returns the canonical namespace/name key used in logs and messages.
func NamespacedBindingKey(namespace, name string) string {
	return types.NamespacedName{Namespace: namespace, Name: name}.String()
}

// EffectiveMode applies the API default for InferenceIdentityBinding mode.
func EffectiveMode(mode kleymv1alpha1.InferenceIdentityBindingMode) kleymv1alpha1.InferenceIdentityBindingMode {
	if mode == "" {
		return kleymv1alpha1.InferenceIdentityBindingModePerObjective
	}
	return mode
}

// BindingPoolRef normalizes the binding's required pool anchor.
func BindingPoolRef(binding *kleymv1alpha1.InferenceIdentityBinding) (PoolRef, error) {
	name := strings.TrimSpace(binding.Spec.PoolRef.Name)
	if name == "" {
		return PoolRef{}, fmt.Errorf("spec.poolRef.name is required")
	}

	group := strings.TrimSpace(binding.Spec.PoolRef.Group)
	if group != "" && !IsSupportedInferencePoolGroup(group) {
		return PoolRef{}, fmt.Errorf("spec.poolRef.group %q is not a supported GAIE InferencePool group", group)
	}

	return PoolRef{
		Name:      name,
		Group:     group,
		Namespace: binding.Namespace,
	}, nil
}

// BindingObjectiveRef normalizes the optional objective subject.
func BindingObjectiveRef(
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (ObjectiveRef, bool, error) {
	if binding.Spec.ObjectiveRef == nil {
		return ObjectiveRef{}, false, nil
	}

	name := strings.TrimSpace(binding.Spec.ObjectiveRef.Name)
	if name == "" {
		return ObjectiveRef{}, true, fmt.Errorf("spec.objectiveRef.name is required")
	}

	group := strings.TrimSpace(binding.Spec.ObjectiveRef.Group)
	if group != "" && !IsSupportedInferenceObjectiveGroup(group) {
		return ObjectiveRef{}, true, fmt.Errorf("spec.objectiveRef.group %q is not a supported GAIE InferenceObjective group", group)
	}

	return ObjectiveRef{
		Name:      name,
		Group:     group,
		Namespace: binding.Namespace,
	}, true, nil
}
