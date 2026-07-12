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

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

type bindingConflictState struct {
	conflicts []identity.VariantConflict
	members   []*kleymv1alpha1.InferenceIdentityBinding
	blocked   bool
}

// evaluateBindingConflicts resolves every valid binding in the namespace before
// the current binding may create or update managed output.
func (r *InferenceIdentityBindingReconciler) evaluateBindingConflicts(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	currentIdentity renderedIdentity,
) (bindingConflictState, error) {
	bindings := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindings, client.InNamespace(binding.Namespace)); err != nil {
		return bindingConflictState{}, err
	}

	records := make([]identity.VariantRecord, 0, len(bindings.Items))
	bindingsByRef := make(map[types.NamespacedName]*kleymv1alpha1.InferenceIdentityBinding, len(bindings.Items))
	blockedMembers := make([]*kleymv1alpha1.InferenceIdentityBinding, 0)
	for index := range bindings.Items {
		peer := bindings.Items[index].DeepCopy()
		plan, relevant, blocksRecovery, err := r.identityForConflictEvaluation(ctx, binding, currentIdentity, peer)
		if err != nil {
			return bindingConflictState{}, err
		}
		if blocksRecovery {
			blockedMembers = append(blockedMembers, peer)
			continue
		}
		if !relevant {
			continue
		}
		ref := types.NamespacedName{Namespace: peer.Namespace, Name: peer.Name}
		bindingsByRef[ref] = peer
		records = append(records, variantRecord(peer, plan))
	}

	state := currentBindingConflictState(binding, records, bindingsByRef)
	if len(blockedMembers) > 0 {
		state.blocked = true
		state.members = appendConflictMember(state.members, binding)
		for _, peer := range blockedMembers {
			state.members = appendConflictMember(state.members, peer)
		}
	}
	return state, nil
}

// identityForConflictEvaluation renders a peer or conservatively blocks on
// still-present output whose current binding can no longer be rendered.
func (r *InferenceIdentityBindingReconciler) identityForConflictEvaluation(
	ctx context.Context,
	current *kleymv1alpha1.InferenceIdentityBinding,
	currentIdentity renderedIdentity,
	peer *kleymv1alpha1.InferenceIdentityBinding,
) (renderedIdentity, bool, bool, error) {
	if !peer.DeletionTimestamp.IsZero() {
		remaining, err := r.listManagedClusterSPIFFEIDs(ctx, peer)
		if err != nil {
			return renderedIdentity{}, false, false, err
		}
		if len(remaining) == 0 {
			return renderedIdentity{}, false, false, nil
		}
	}
	if peer.Namespace == current.Namespace && peer.Name == current.Name {
		return currentIdentity, true, false, nil
	}
	plan, err := r.renderIdentityForBinding(ctx, peer)
	if err == nil {
		return plan, true, false, nil
	}
	var stateErr reconcileStateError
	if errorsAsStateError(err, &stateErr) {
		remaining, listErr := r.listManagedClusterSPIFFEIDs(ctx, peer)
		if listErr != nil {
			return renderedIdentity{}, false, false, listErr
		}
		return renderedIdentity{}, false, len(remaining) > 0, nil
	}
	return renderedIdentity{}, false, false, err
}

// variantRecord projects validated render state into the pure evaluator input.
func variantRecord(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	plan renderedIdentity,
) identity.VariantRecord {
	return identity.VariantRecord{
		BindingRef:         types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name},
		ServiceAccountName: binding.Spec.ServiceAccountName,
		Variant:            plan.Variant,
		SpiffeID:           plan.SpiffeID,
	}
}

// currentBindingConflictState selects the current binding's directional
// diagnoses and deterministic managed-output withdrawal set.
func currentBindingConflictState(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	records []identity.VariantRecord,
	bindingsByRef map[types.NamespacedName]*kleymv1alpha1.InferenceIdentityBinding,
) bindingConflictState {
	currentRef := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}
	state := bindingConflictState{}
	for _, conflict := range identity.EvaluateVariantConflicts(records) {
		if conflict.BindingRef != currentRef {
			continue
		}
		state.conflicts = append(state.conflicts, conflict)
	}
	if len(state.conflicts) == 0 {
		return state
	}
	state.members = appendConflictMember(state.members, bindingsByRef[currentRef])
	for _, conflict := range state.conflicts {
		if member := bindingsByRef[conflict.PeerBindingRef]; member != nil {
			state.members = appendConflictMember(state.members, member)
		}
	}
	return state
}

// appendConflictMember keeps the withdrawal set unique without losing its
// evaluator-defined order.
func appendConflictMember(
	members []*kleymv1alpha1.InferenceIdentityBinding,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) []*kleymv1alpha1.InferenceIdentityBinding {
	if binding == nil {
		return members
	}
	for _, member := range members {
		if member.Namespace == binding.Namespace && member.Name == binding.Name {
			return members
		}
	}
	return append(members, binding)
}

// withdrawConflictOutputs removes all managed output for the current conflict
// set and confirms absence before the caller may settle Conflict=True.
func (r *InferenceIdentityBindingReconciler) withdrawConflictOutputs(
	ctx context.Context,
	members []*kleymv1alpha1.InferenceIdentityBinding,
) (bool, error) {
	for _, member := range members {
		if err := r.cleanupManagedClusterSPIFFEIDs(ctx, member); err != nil {
			return false, err
		}
	}
	for _, member := range members {
		remaining, err := r.listManagedClusterSPIFFEIDs(ctx, member)
		if err != nil {
			return false, err
		}
		if len(remaining) > 0 {
			return false, nil
		}
	}
	return true, nil
}

// reconcileConflictState withdraws conflict-member output before publishing a
// settled conflict result, so Ready never claims safe output absence early.
func (r *InferenceIdentityBindingReconciler) reconcileConflictState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	currentIdentity renderedIdentity,
) (bool, ctrl.Result, error) {
	conflictState, err := r.evaluateBindingConflicts(ctx, binding, currentIdentity)
	if err != nil {
		if statusErr := r.patchManagedOutputApplyFailureStatus(ctx, binding, err); statusErr != nil {
			return true, ctrl.Result{}, statusErr
		}
		return true, ctrl.Result{}, err
	}
	if len(conflictState.conflicts) == 0 && !conflictState.blocked {
		return false, ctrl.Result{}, nil
	}

	outputsAbsent, err := r.withdrawConflictOutputs(ctx, conflictState.members)
	if err != nil {
		if statusErr := r.patchManagedOutputApplyFailureStatus(ctx, binding, err); statusErr != nil {
			return true, ctrl.Result{}, statusErr
		}
		return true, ctrl.Result{}, err
	}
	if !outputsAbsent || conflictState.blocked {
		applyPendingConflictStatus(&binding.Status, binding.Generation)
		if err := r.patchStatus(ctx, binding); err != nil {
			return true, ctrl.Result{}, err
		}
		return true, ctrl.Result{RequeueAfter: deleteVerificationRequeueAfter}, nil
	}

	applyConflictStatus(&binding.Status, binding.Generation, conflictState.conflicts)
	if err := r.patchStatus(ctx, binding); err != nil {
		return true, ctrl.Result{}, err
	}
	r.recordTerminalOutcome(binding)
	return true, ctrl.Result{}, nil
}
