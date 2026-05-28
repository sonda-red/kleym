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
package gaie

import (
	"fmt"
	"strings"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	// ConditionTypeInvalidRef matches the controller status condition for invalid input references.
	ConditionTypeInvalidRef = "InvalidRef"
)

// StateError carries condition metadata for shared GAIE computation errors.
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
