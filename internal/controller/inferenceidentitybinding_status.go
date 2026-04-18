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
	"context"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func initializeConditions(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
) {
	canonical := []struct {
		conditionType string
		message       string
	}{
		{conditionTypeReady, "Readiness has not been evaluated yet"},
		{conditionTypeConflict, "Identity collision has not been evaluated yet"},
		{conditionTypeInvalidRef, "Reference validity has not been evaluated yet"},
		{conditionTypeUnsafeSelector, "Selector safety has not been evaluated yet"},
		{conditionTypeRenderFailure, "Render health has not been evaluated yet"},
	}

	for _, entry := range canonical {
		conditionStatus := metav1.ConditionUnknown
		reason := "Initializing"
		message := entry.message

		existing := meta.FindStatusCondition(status.Conditions, entry.conditionType)
		if existing != nil {
			conditionStatus = existing.Status
			if strings.TrimSpace(existing.Reason) != "" {
				reason = existing.Reason
			}
			if strings.TrimSpace(existing.Message) != "" {
				message = existing.Message
			}
		}

		setCondition(status, generation, entry.conditionType, conditionStatus, reason, message)
	}
}

func applySuccessStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	identities []renderedIdentity,
) {
	status.ComputedSpiffeIDs = make([]kleymv1alpha1.ComputedSpiffeIDStatus, 0, len(identities))
	status.RenderedSelectors = make([]kleymv1alpha1.RenderedSelectorStatus, 0, len(identities))

	for _, identity := range identities {
		status.ComputedSpiffeIDs = append(status.ComputedSpiffeIDs, kleymv1alpha1.ComputedSpiffeIDStatus{
			Mode:     identity.Mode,
			SpiffeID: identity.SpiffeID,
		})
		status.RenderedSelectors = append(status.RenderedSelectors, kleymv1alpha1.RenderedSelectorStatus{
			SpiffeID:  identity.SpiffeID,
			Selectors: identity.Selectors,
		})
	}

	setCondition(status, generation, conditionTypeReady, metav1.ConditionTrue, "Reconciled", "Binding reconciled")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", noIdentityCollisionMessage)
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
}

func applyFailureStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	stateErr *reconcileStateError,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil

	setCondition(status, generation, conditionTypeReady, metav1.ConditionFalse, stateErr.reason, stateErr.message)
	setCondition(status, generation, stateErr.conditionType, metav1.ConditionTrue, stateErr.reason, stateErr.message)

	if stateErr.conditionType != conditionTypeInvalidRef {
		setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
	}
	if stateErr.conditionType != conditionTypeConflict {
		setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", noIdentityCollisionMessage)
	}
	if stateErr.conditionType != conditionTypeUnsafeSelector {
		setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
	}
	if stateErr.conditionType != conditionTypeRenderFailure {
		setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
	}
}

func applyCollisionStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	hasCollision bool,
	message string,
) {
	if hasCollision {
		status.ComputedSpiffeIDs = nil
		status.RenderedSelectors = nil
		setCondition(status, generation, conditionTypeReady, metav1.ConditionFalse, "IdentityCollision", message)
		setCondition(status, generation, conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision", message)
		setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
		setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
		setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
		return
	}

	if strings.TrimSpace(message) == "" {
		message = noIdentityCollisionMessage
	}
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", message)
}

func conditionIsTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := meta.FindStatusCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func (r *InferenceIdentityBindingReconciler) patchStatusFromBase(
	ctx context.Context,
	base *kleymv1alpha1.InferenceIdentityBinding,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	if reflect.DeepEqual(base.Status, binding.Status) {
		return nil
	}
	return r.Status().Patch(ctx, binding, client.MergeFrom(base))
}

func setCondition(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	conditionType string,
	conditionStatus metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	})
}

func (r *InferenceIdentityBindingReconciler) patchStatus(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	mutate func(status *kleymv1alpha1.InferenceIdentityBindingStatus),
) error {
	base := binding.DeepCopy()
	mutate(&binding.Status)
	if reflect.DeepEqual(base.Status, binding.Status) {
		return nil
	}

	return r.Status().Patch(ctx, binding, client.MergeFrom(base))
}
