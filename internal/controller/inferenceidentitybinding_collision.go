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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// computePerObjectiveCollisionSet detects identity collisions between PerObjective bindings.
//
// Two bindings collide when they produce identical (podSelector, selectors,
// containerDiscriminatorType, containerDiscriminatorValue) tuples — meaning
// SPIRE would assign the same identity to the same container twice.
//
// Algorithm:
//  1. Gather candidate bindings that could collide with the current one.
//  2. Compute a JSON fingerprint for each candidate's rendered identity.
//  3. Group candidates by fingerprint — groups with 2+ members are collisions.
//  4. Mark every member of a collision group; the controller will set the
//     Conflict condition on all of them and block ClusterSPIFFEID reconciliation
//     until the collision is resolved (e.g. by changing a container discriminator).
//
// See docs/design/collision-detection.md for the full design.
func (r *InferenceIdentityBindingReconciler) computePerObjectiveCollisionSet(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
	wasCurrentColliding bool,
) (perObjectiveCollisionSet, error) {
	collisionSet := perObjectiveCollisionSet{
		currentMessage: noIdentityCollisionMessage,
	}

	candidateBindings, err := r.listCollisionCandidateBindings(ctx, binding, wasCurrentColliding)
	if err != nil {
		return perObjectiveCollisionSet{}, err
	}

	candidates := make([]perObjectiveCollisionCandidate, 0, len(candidateBindings))
	currentBindingKey := namespacedBindingKey(binding.Namespace, binding.Name)
	for i := range candidateBindings {
		candidateBinding := candidateBindings[i]
		if !candidateBinding.DeletionTimestamp.IsZero() {
			continue
		}
		if effectiveMode(candidateBinding.Spec.Mode) != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
			continue
		}

		candidateKey := namespacedBindingKey(candidateBinding.Namespace, candidateBinding.Name)
		var candidateIdentity renderedIdentity
		if candidateKey == currentBindingKey {
			if identity.Mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
				continue
			}
			candidateIdentity = identity
		} else {
			resolvedIdentity, resolveErr := r.renderIdentityForBinding(ctx, candidateBinding)
			if resolveErr != nil {
				continue
			}
			candidateIdentity = resolvedIdentity
		}

		fingerprint, fingerprintErr := perObjectiveCollisionFingerprint(candidateIdentity, candidateBinding.Spec.ContainerDiscriminator)
		if fingerprintErr != nil {
			continue
		}

		candidates = append(candidates, perObjectiveCollisionCandidate{
			binding: candidateBinding,
			key:     fingerprint,
		})
	}

	groups := make(map[string][]int, len(candidates))
	for i, candidate := range candidates {
		groups[candidate.key] = append(groups[candidate.key], i)
	}

	collidingByBinding := make(map[string]bool, len(candidates))
	messageByBinding := make(map[string]string, len(candidates))
	for _, indexes := range groups {
		if len(indexes) < 2 {
			for _, idx := range indexes {
				candidate := candidates[idx]
				messageByBinding[namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)] = noIdentityCollisionMessage
			}
			continue
		}

		memberNames := make([]string, 0, len(indexes))
		for _, idx := range indexes {
			memberNames = append(memberNames, candidates[idx].binding.Name)
		}
		sort.Strings(memberNames)

		for _, idx := range indexes {
			candidate := candidates[idx]
			bindingKey := namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)
			collidingByBinding[bindingKey] = true
			messageByBinding[bindingKey] = identityCollisionMessage(candidate.binding.Name, memberNames)
		}
	}

	collisionSet.states = make([]perObjectiveCollisionState, 0, len(candidates))
	for i := range candidates {
		candidate := candidates[i]
		bindingKey := namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)
		message := messageByBinding[bindingKey]
		if message == "" {
			message = noIdentityCollisionMessage
		}
		hasCollision := collidingByBinding[bindingKey]

		collisionSet.states = append(collisionSet.states, perObjectiveCollisionState{
			binding:      candidate.binding,
			hasCollision: hasCollision,
			message:      message,
		})

		if bindingKey == currentBindingKey {
			collisionSet.currentHasCollision = hasCollision
			collisionSet.currentMessage = message
		}
	}

	return collisionSet, nil
}

// listCollisionCandidateBindings assembles the set of bindings that could
// collide with the current binding.
//
// It uses two strategies to keep the candidate set small:
//  1. If the current binding is PerObjective, look up bindings with the same
//     container discriminator key using a field index (fast, narrow).
//  2. If the current binding was previously colliding, look up the named peers
//     from the Conflict condition message. If no peer names are available
//     (message format mismatch or condition cleared externally), fall back to
//     listing all PerObjective bindings in the namespace.
//
// The results are deduplicated by namespace/name and sorted for deterministic
// reconciliation order.
func (r *InferenceIdentityBindingReconciler) listCollisionCandidateBindings(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	wasCurrentColliding bool,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	candidatesByKey := map[string]*kleymv1alpha1.InferenceIdentityBinding{}
	addCandidate := func(candidate *kleymv1alpha1.InferenceIdentityBinding) {
		if candidate == nil {
			return
		}
		bindingKey := namespacedBindingKey(candidate.Namespace, candidate.Name)
		candidatesByKey[bindingKey] = candidate
	}

	if effectiveMode(binding.Spec.Mode) == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		discriminatorKey := containerDiscriminatorIndexKey(binding.Spec.ContainerDiscriminator)
		matchingDiscriminatorBindings, err := r.listBindingsByField(
			ctx,
			binding.Namespace,
			fieldIndexContainerDiscriminatorKey,
			discriminatorKey,
		)
		if err != nil {
			return nil, err
		}
		for i := range matchingDiscriminatorBindings {
			addCandidate(matchingDiscriminatorBindings[i])
		}
		addCandidate(binding.DeepCopy())
	}

	if wasCurrentColliding {
		peerNames := collisionPeerBindingNames(binding.Status.Conditions)
		if len(peerNames) == 0 {
			perObjectiveBindings, err := r.listBindingsByField(
				ctx,
				binding.Namespace,
				fieldIndexEffectiveMode,
				modeValuePerObjective,
			)
			if err != nil {
				return nil, err
			}
			for i := range perObjectiveBindings {
				addCandidate(perObjectiveBindings[i])
			}
		} else {
			for _, peerName := range peerNames {
				peer := &kleymv1alpha1.InferenceIdentityBinding{}
				if err := r.Get(
					ctx,
					types.NamespacedName{Namespace: binding.Namespace, Name: peerName},
					peer,
				); err != nil {
					if apierrors.IsNotFound(err) {
						continue
					}
					return nil, err
				}
				addCandidate(peer)
			}
		}
	}

	candidateKeys := make([]string, 0, len(candidatesByKey))
	for key := range candidatesByKey {
		candidateKeys = append(candidateKeys, key)
	}
	sort.Strings(candidateKeys)

	candidates := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(candidateKeys))
	for _, key := range candidateKeys {
		candidates = append(candidates, candidatesByKey[key])
	}

	return candidates, nil
}

// listBindingsByField looks up bindings using a controller-runtime field index.
// Field indexes allow efficient lookups without a full namespace scan. If the
// index is not available (e.g. during envtest bootstrap before indexes are
// registered, or during partial CRD installation), the lookup falls back to
// listing all bindings in the namespace and filtering in memory.
func (r *InferenceIdentityBindingReconciler) listBindingsByField(
	ctx context.Context,
	namespace string,
	field string,
	value string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(
		ctx,
		bindingList,
		client.InNamespace(namespace),
		client.MatchingFields{field: value},
	); err != nil {
		if !isFieldLookupUnsupported(err) {
			return nil, err
		}
		return r.listBindingsByFieldFallback(ctx, namespace, field, value)
	}

	result := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		result = append(result, bindingList.Items[i].DeepCopy())
	}

	return result, nil
}

func (r *InferenceIdentityBindingReconciler) listBindingsByFieldFallback(
	ctx context.Context,
	namespace string,
	field string,
	value string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	result := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		binding := bindingList.Items[i].DeepCopy()
		if bindingMatchesField(binding, field, value) {
			result = append(result, binding)
		}
	}

	return result, nil
}

func bindingMatchesField(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	field string,
	value string,
) bool {
	switch field {
	case fieldIndexTargetRefName:
		return strings.TrimSpace(binding.Spec.TargetRef.Name) == value
	case fieldIndexEffectiveMode:
		return string(effectiveMode(binding.Spec.Mode)) == value
	case fieldIndexContainerDiscriminatorKey:
		return containerDiscriminatorIndexKey(binding.Spec.ContainerDiscriminator) == value
	default:
		return false
	}
}

func isFieldLookupUnsupported(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "index with name") ||
		strings.Contains(errText, "field label not supported")
}

// collisionPeerBindingNames extracts peer binding names from the Conflict
// condition message.
//
// Peer names are encoded in the condition message text rather than in a
// dedicated status field. This is a deliberate trade-off: a structured
// status.collisionPeers field would require an API type change, CRD
// regeneration, and migration logic for a condition that is rare in practice.
// The message-based approach is self-healing — if the message format is
// unrecognizable, the caller falls back to scanning all PerObjective bindings
// in the namespace on the next reconcile.
func collisionPeerBindingNames(conditions []metav1.Condition) []string {
	conflictCondition := meta.FindStatusCondition(conditions, conditionTypeConflict)
	if conflictCondition == nil || conflictCondition.Status != metav1.ConditionTrue {
		return nil
	}

	message := strings.TrimSpace(conflictCondition.Message)
	if !strings.HasPrefix(message, identityCollisionMessagePrefix) {
		return nil
	}
	if !strings.HasSuffix(message, identityCollisionMessageSuffix) {
		return nil
	}

	peerList := strings.TrimPrefix(message, identityCollisionMessagePrefix)
	peerList = strings.TrimSuffix(peerList, identityCollisionMessageSuffix)
	peerList = strings.TrimSpace(peerList)
	if peerList == "" {
		return nil
	}

	seen := map[string]struct{}{}
	peers := []string{}
	for _, entry := range strings.Split(peerList, ",") {
		peerName := strings.TrimSpace(entry)
		if peerName == "" {
			continue
		}
		if _, exists := seen[peerName]; exists {
			continue
		}
		seen[peerName] = struct{}{}
		peers = append(peers, peerName)
	}
	sort.Strings(peers)

	return peers
}

// perObjectiveCollisionFingerprint produces a deterministic string key for a
// rendered identity. Two identities with the same fingerprint would produce
// overlapping ClusterSPIFFEID resources targeting the same container, which is
// unsafe. The key is: podSelector JSON | selectors JSON | discriminator type | discriminator value.
func perObjectiveCollisionFingerprint(
	identity renderedIdentity,
	discriminator *kleymv1alpha1.ContainerDiscriminator,
) (string, error) {
	if discriminator == nil {
		return "", fmt.Errorf("containerDiscriminator is required for per-objective collision detection")
	}

	containerValue := strings.TrimSpace(discriminator.Value)
	if containerValue == "" {
		return "", fmt.Errorf("containerDiscriminator.value must not be empty")
	}

	podSelectorFingerprint, err := normalizedPodSelectorFingerprint(identity.PodSelector)
	if err != nil {
		return "", err
	}

	selectorFingerprint, err := normalizedSelectorFingerprint(identity.Selectors)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s|%s|%s|%s", podSelectorFingerprint, selectorFingerprint, discriminator.Type, containerValue), nil
}

func normalizedPodSelectorFingerprint(selector map[string]any) (string, error) {
	if len(selector) == 0 {
		return "", fmt.Errorf("pod selector must be present for collision detection")
	}

	serialized, err := json.Marshal(selector)
	if err != nil {
		return "", fmt.Errorf("failed to encode pod selector fingerprint: %w", err)
	}

	return string(serialized), nil
}

func normalizedSelectorFingerprint(selectors []string) (string, error) {
	if len(selectors) == 0 {
		return "", fmt.Errorf("selectors must be present for collision detection")
	}

	serialized, err := json.Marshal(selectors)
	if err != nil {
		return "", fmt.Errorf("failed to encode selector fingerprint: %w", err)
	}

	return string(serialized), nil
}

func identityCollisionMessage(bindingName string, collidingBindings []string) string {
	peers := make([]string, 0, len(collidingBindings))
	for _, name := range collidingBindings {
		if name == bindingName {
			continue
		}
		peers = append(peers, name)
	}
	sort.Strings(peers)
	return identityCollisionMessagePrefix + strings.Join(peers, ", ") + identityCollisionMessageSuffix
}
