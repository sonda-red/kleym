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
package controller

import (
	"errors"

	"k8s.io/apimachinery/pkg/types"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// reconcileStateError bundles a Kubernetes condition type, reason, and message
// into a single error value. This lets helper functions (resolver, render,
// collision) return domain-specific errors that the Reconcile() caller can
// convert directly into the right status condition, without scattering
// condition-setting logic across multiple files.
//
// Any error that is NOT a reconcileStateError is treated as an unexpected
// failure and returned to the controller-runtime for generic retry.
type reconcileStateError struct {
	conditionType string
	reason        string
	message       string
}

func (e *reconcileStateError) Error() string {
	return e.message
}

func newStateError(conditionType, reason, message string) *reconcileStateError {
	return &reconcileStateError{
		conditionType: conditionType,
		reason:        reason,
		message:       message,
	}
}

type inferencePoolRef struct {
	Name      string
	Group     string
	Namespace string
}

type renderedIdentity struct {
	Name         string
	Mode         kleymv1alpha1.InferenceIdentityBindingMode
	SpiffeID     string
	Selectors    []string
	PodSelector  map[string]any
	ObjectiveRef string
	PoolRef      string
}

type renderTemplateData struct {
	Namespace                   string
	BindingName                 string
	ObjectiveName               string
	PoolName                    string
	Mode                        string
	ContainerDiscriminatorType  string
	ContainerDiscriminatorValue string
}

type desiredBindingState struct {
	identities               []renderedIdentity
	perObjectiveCollisionSet perObjectiveCollisionSet
}

type collisionApplyResult struct {
	currentHasCollision bool
	currentMessage      string
	currentDetected     bool
	currentResolved     bool
}

type perObjectiveCollisionCandidate struct {
	binding *kleymv1alpha1.InferenceIdentityBinding
	key     string
}

type perObjectiveCollisionState struct {
	binding      *kleymv1alpha1.InferenceIdentityBinding
	hasCollision bool
	message      string
}

type perObjectiveCollisionSet struct {
	states              []perObjectiveCollisionState
	currentHasCollision bool
	currentMessage      string
}

func namespacedBindingKey(namespace, name string) string {
	return types.NamespacedName{Namespace: namespace, Name: name}.String()
}

func effectiveMode(mode kleymv1alpha1.InferenceIdentityBindingMode) kleymv1alpha1.InferenceIdentityBindingMode {
	if mode == "" {
		return kleymv1alpha1.InferenceIdentityBindingModePerObjective
	}
	return mode
}

// errorsAsStateError is a type-assertion helper that extracts a
// reconcileStateError from a generic error. It works like errors.As but
// for a concrete pointer type (not an interface), copying the value into
// the caller's target so the caller can read conditionType/reason/message.
func errorsAsStateError(err error, target *reconcileStateError) bool {
	var stateErr *reconcileStateError
	if !errors.As(err, &stateErr) {
		return false
	}
	*target = *stateErr
	return true
}
