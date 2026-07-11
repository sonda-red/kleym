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
	"github.com/sonda-red/kleym/internal/identity"
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
		{conditionTypeInvalidRef, "Reference validity has not been evaluated yet"},
		{conditionTypeUnsafeSelector, "Selector safety has not been evaluated yet"},
		{conditionTypeConflict, "Identity boundary conflicts have not been evaluated yet"},
		{conditionTypeRenderFailure, "Render health has not been evaluated yet"},
	}

	for _, entry := range canonical {
		conditionStatus := metav1.ConditionUnknown
		reason := conditionReasonInitializing
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

// applyIdentityBoundaryStatus retains validated boundary data for diagnosis.
func applyIdentityBoundaryStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	boundary identity.Boundary,
) {
	status.IdentityBoundary = &kleymv1alpha1.IdentityBoundaryStatus{
		LabelKey:   boundary.LabelKey,
		LabelValue: boundary.LabelValue,
	}
}

// applyOperatorConfig records the operator render settings used for this status update.
func applyOperatorConfig(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	config OperatorConfig,
) {
	status.TrustDomain = config.TrustDomain
	status.ClusterSPIFFEIDClassName = config.ClusterSPIFFEIDClassName
}

func applySuccessStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	identities []renderedIdentity,
	managedStatuses []kleymv1alpha1.RenderedClusterSPIFFEIDStatus,
) {
	status.ComputedSpiffeIDs = make([]kleymv1alpha1.ComputedSpiffeIDStatus, 0, len(identities))
	status.RenderedSelectors = make([]kleymv1alpha1.RenderedSelectorStatus, 0, len(identities))
	status.RenderedClusterSPIFFEID = nil
	status.Conflicts = nil

	for _, rendered := range identities {
		status.ComputedSpiffeIDs = append(status.ComputedSpiffeIDs, kleymv1alpha1.ComputedSpiffeIDStatus{
			SpiffeID: rendered.SpiffeID,
		})
		status.RenderedSelectors = append(status.RenderedSelectors, kleymv1alpha1.RenderedSelectorStatus{
			SpiffeID:  rendered.SpiffeID,
			Selectors: rendered.Selectors,
		})
	}
	if len(managedStatuses) > 0 {
		rendered := managedStatuses[0]
		status.RenderedClusterSPIFFEID = &rendered
		status.PendingClusterSPIFFEIDName = ""
		status.OwnedClusterSPIFFEIDName = rendered.Name
	}

	setCondition(status, generation, conditionTypeReady, metav1.ConditionTrue, conditionReasonReconciled, "Binding reconciled")
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved, "Identity boundary is exclusive")
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
}

// applyConflictStatus projects deterministic evaluator results only after
// managed output absence has been confirmed.
func applyConflictStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	conflicts []identity.BoundaryConflict,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil
	status.RenderedClusterSPIFFEID = nil
	status.PendingClusterSPIFFEIDName = ""
	status.OwnedClusterSPIFFEIDName = ""
	status.Conflicts = make([]kleymv1alpha1.IdentityBoundaryConflictStatus, 0, len(conflicts))
	reason := conditionReasonIdentityBoundaryConflict
	for _, conflict := range conflicts {
		if conflict.Cause == identity.CauseDuplicateSPIFFEID {
			reason = conditionReasonDuplicateIdentityBinding
		}
		status.Conflicts = append(status.Conflicts, kleymv1alpha1.IdentityBoundaryConflictStatus{
			BindingRef: kleymv1alpha1.BindingReference{
				Namespace: conflict.PeerBindingRef.Namespace,
				Name:      conflict.PeerBindingRef.Name,
			},
			Cause:    string(conflict.Cause),
			SpiffeID: conflict.PeerSpiffeID,
			LabelKey: conflict.PeerLabelKey,
			Value:    conflict.PeerLabelValue,
		})
	}
	message := "Identity boundary conflicts with one or more peer bindings"
	setCondition(status, generation, conditionTypeReady, metav1.ConditionFalse, reason, message)
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionTrue, reason, message)
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
}

// applyPendingManagedOutputClaimStatus persists the deterministic name before
// Create so a retry can identify output created before the success status patch.
func applyPendingManagedOutputClaimStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	name string,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil
	status.RenderedClusterSPIFFEID = nil
	status.Conflicts = nil
	status.PendingClusterSPIFFEIDName = name
	message := "Waiting to confirm managed output creation"
	setCondition(status, generation, conditionTypeReady, metav1.ConditionUnknown, conditionReasonInitializing, message)
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved, "Identity boundary is exclusive")
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
}

// applyPendingConflictStatus clears stale rendered output without claiming the
// conflict is settled before managed output absence has been confirmed.
func applyPendingConflictStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil
	status.RenderedClusterSPIFFEID = nil
	status.Conflicts = nil
	setCondition(status, generation, conditionTypeReady, metav1.ConditionUnknown, conditionReasonInitializing, "Waiting for managed output absence")
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionUnknown, conditionReasonInitializing, "Waiting for managed output absence")
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
}

// applyPendingManagedOutputReplacementStatus avoids reporting either the old
// or desired output as ready while the recorded old output is terminating.
func applyPendingManagedOutputReplacementStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil
	status.RenderedClusterSPIFFEID = nil
	status.Conflicts = nil
	message := "Waiting for previous managed output absence before creating its replacement"
	setCondition(status, generation, conditionTypeReady, metav1.ConditionUnknown, conditionReasonInitializing, message)
	setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved, "Identity boundary is exclusive")
	setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
}

func applyFailureStatus(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	stateErr *reconcileStateError,
) {
	status.ComputedSpiffeIDs = nil
	status.RenderedSelectors = nil
	status.RenderedClusterSPIFFEID = nil
	status.Conflicts = nil

	setCondition(status, generation, conditionTypeReady, metav1.ConditionFalse, stateErr.reason, stateErr.message)
	setCondition(status, generation, stateErr.conditionType, metav1.ConditionTrue, stateErr.reason, stateErr.message)

	if stateErr.conditionType != conditionTypeInvalidRef {
		setCondition(status, generation, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved, "References are valid")
	}
	if stateErr.conditionType != conditionTypeUnsafeSelector {
		setCondition(status, generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved, "Selectors are safe")
	}
	if stateErr.conditionType != conditionTypeConflict {
		setCondition(status, generation, conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved, "Identity boundary is exclusive")
	}
	if stateErr.conditionType != conditionTypeRenderFailure {
		setCondition(status, generation, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved, "Rendering is healthy")
	}
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
