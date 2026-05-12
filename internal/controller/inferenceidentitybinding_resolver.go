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
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func (r *InferenceIdentityBindingReconciler) resolveInferenceObjective(
	ctx context.Context,
	objectiveRef inferenceObjectiveRef,
) (*unstructured.Unstructured, error) {
	objectiveCandidates := candidateObjectiveGVKs(r.resolveObjectiveGVKs(), objectiveRef.Group)
	objective, crdMissing, err := r.resolveByCandidates(
		ctx,
		types.NamespacedName{Namespace: objectiveRef.Namespace, Name: objectiveRef.Name},
		objectiveCandidates,
	)
	if err != nil {
		return nil, err
	}
	if objective != nil {
		return objective, nil
	}
	if crdMissing {
		return nil, newStateError(
			conditionTypeInvalidRef,
			"InferenceObjectiveCRDMissing",
			"InferenceObjective CRD is not installed",
		)
	}
	return nil, newStateError(
		conditionTypeInvalidRef,
		"TargetObjectiveNotFound",
		fmt.Sprintf("objectiveRef %q was not found", objectiveRef.Name),
	)
}

func (r *InferenceIdentityBindingReconciler) resolveInferencePool(
	ctx context.Context,
	poolRef inferencePoolRef,
) (*unstructured.Unstructured, error) {
	poolCandidates := candidatePoolGVKs(r.resolvePoolGVKs(), poolRef.Group)
	pool, crdMissing, err := r.resolveByCandidates(
		ctx,
		types.NamespacedName{Namespace: poolRef.Namespace, Name: poolRef.Name},
		poolCandidates,
	)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		return pool, nil
	}
	if crdMissing {
		return nil, newStateError(
			conditionTypeInvalidRef,
			"InferencePoolCRDMissing",
			"InferencePool CRD is not installed",
		)
	}
	return nil, newStateError(
		conditionTypeInvalidRef,
		"TargetPoolNotFound",
		fmt.Sprintf("poolRef %q was not found", poolRef.Name),
	)
}

func (r *InferenceIdentityBindingReconciler) resolveByCandidates(
	ctx context.Context,
	key types.NamespacedName,
	candidates []schema.GroupVersionKind,
) (*unstructured.Unstructured, bool, error) {
	crdMissing := false

	for _, gvk := range candidates {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		err := r.Get(ctx, key, obj)
		switch {
		case err == nil:
			return obj, crdMissing, nil
		case apierrors.IsNotFound(err):
			continue
		case meta.IsNoMatchError(err):
			crdMissing = true
			continue
		default:
			return nil, crdMissing, err
		}
	}

	return nil, crdMissing, nil
}

// bindingPoolRef normalizes the binding's required pool anchor into the
// resolver's internal shape so reconciliation always starts from pool-derived
// selector provenance.
func bindingPoolRef(binding *kleymv1alpha1.InferenceIdentityBinding) (inferencePoolRef, error) {
	name := strings.TrimSpace(binding.Spec.PoolRef.Name)
	if name == "" {
		return inferencePoolRef{}, fmt.Errorf("spec.poolRef.name is required")
	}

	group := strings.TrimSpace(binding.Spec.PoolRef.Group)
	if group != "" && !isSupportedInferencePoolGroup(group) {
		return inferencePoolRef{}, fmt.Errorf("spec.poolRef.group %q is not a supported GAIE InferencePool group", group)
	}

	return inferencePoolRef{
		Name:      name,
		Group:     group,
		Namespace: binding.Namespace,
	}, nil
}

// bindingObjectiveRef normalizes the optional objective subject without making
// PoolOnly reconciliation depend on GAIE's alpha objective API.
func bindingObjectiveRef(
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (inferenceObjectiveRef, bool, error) {
	if binding.Spec.ObjectiveRef == nil {
		return inferenceObjectiveRef{}, false, nil
	}

	name := strings.TrimSpace(binding.Spec.ObjectiveRef.Name)
	if name == "" {
		return inferenceObjectiveRef{}, true, fmt.Errorf("spec.objectiveRef.name is required")
	}

	group := strings.TrimSpace(binding.Spec.ObjectiveRef.Group)
	if group != "" && !isSupportedInferenceObjectiveGroup(group) {
		return inferenceObjectiveRef{}, true, fmt.Errorf("spec.objectiveRef.group %q is not a supported GAIE InferenceObjective group", group)
	}

	return inferenceObjectiveRef{
		Name:      name,
		Group:     group,
		Namespace: binding.Namespace,
	}, true, nil
}

func extractPoolRef(objective *unstructured.Unstructured, defaultNamespace string) (inferencePoolRef, error) {
	poolRefMap, found, err := unstructured.NestedMap(objective.Object, "spec", "poolRef")
	if err != nil {
		return inferencePoolRef{}, fmt.Errorf("failed to decode objective spec.poolRef: %w", err)
	}
	if !found {
		return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef is required")
	}

	name, ok := poolRefMap["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.name is required")
	}

	group := ""
	if rawGroup, exists := poolRefMap["group"]; exists {
		groupValue, groupOK := rawGroup.(string)
		if !groupOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.group must be a string")
		}
		group = strings.TrimSpace(groupValue)
		if group != "" && !isSupportedInferencePoolGroup(group) {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.group %q is not a supported GAIE InferencePool group", group)
		}
	}

	namespace := defaultNamespace
	if rawNamespace, exists := poolRefMap["namespace"]; exists {
		namespaceValue, namespaceOK := rawNamespace.(string)
		if !namespaceOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.namespace must be a string")
		}
		if namespaceValue != "" && namespaceValue != defaultNamespace {
			return inferencePoolRef{}, fmt.Errorf("cross-namespace poolRef is not allowed")
		}
		if namespaceValue != "" {
			namespace = namespaceValue
		}
	}

	if rawKind, exists := poolRefMap["kind"]; exists {
		kindValue, kindOK := rawKind.(string)
		if !kindOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be a string")
		}
		if kindValue != "" && kindValue != "InferencePool" {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be InferencePool")
		}
	}

	return inferencePoolRef{
		Name:      strings.TrimSpace(name),
		Group:     group,
		Namespace: namespace,
	}, nil
}

// validateObjectiveTargetsPool preserves the pool-first contract when an
// objective subject is present: objective-backed identities may use the
// objective name, but selectors must still come from the binding pool.
func validateObjectiveTargetsPool(
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
	defaultNamespace string,
) error {
	objectivePoolRef, err := extractPoolRef(objective, defaultNamespace)
	if err != nil {
		return err
	}

	if objectivePoolRef.Name != pool.GetName() || objectivePoolRef.Namespace != pool.GetNamespace() {
		return fmt.Errorf(
			"objectiveRef %q points at poolRef %q, want %q",
			objective.GetName(),
			namespacedBindingKey(objectivePoolRef.Namespace, objectivePoolRef.Name),
			namespacedBindingKey(pool.GetNamespace(), pool.GetName()),
		)
	}

	if objectivePoolRef.Group != "" && objectivePoolRef.Group != pool.GroupVersionKind().Group {
		return fmt.Errorf(
			"objectiveRef %q points at poolRef group %q, want %q",
			objective.GetName(),
			objectivePoolRef.Group,
			pool.GroupVersionKind().Group,
		)
	}

	return nil
}

// candidateObjectiveGVKs narrows objective lookup to a requested GAIE group
// while still reporting supported-but-missing CRDs as infrastructure readiness
// failures.
func candidateObjectiveGVKs(candidates []schema.GroupVersionKind, group string) []schema.GroupVersionKind {
	if group == "" {
		return candidates
	}

	filtered := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if gvk.Group == group {
			filtered = append(filtered, gvk)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}

	return supportedObjectiveGVKsForGroup(group)
}

func candidatePoolGVKs(candidates []schema.GroupVersionKind, group string) []schema.GroupVersionKind {
	if group == "" {
		return candidates
	}

	filtered := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if gvk.Group == group {
			filtered = append(filtered, gvk)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}

	return supportedPoolGVKsForGroup(group)
}

// isSupportedInferenceObjectiveGroup keeps objectiveRef validation bounded to
// the GAIE groups that kleym has deliberately documented and tested.
func isSupportedInferenceObjectiveGroup(group string) bool {
	return len(supportedObjectiveGVKsForGroup(group)) > 0
}

func isSupportedInferencePoolGroup(group string) bool {
	return len(supportedPoolGVKsForGroup(group)) > 0
}

// supportedObjectiveGVKsForGroup returns static supported objective GVKs so a
// supported group with a missing CRD is reported as infrastructure-not-ready
// instead of being treated as an arbitrary best-effort API lookup.
func supportedObjectiveGVKsForGroup(group string) []schema.GroupVersionKind {
	supported := make([]schema.GroupVersionKind, 0, len(inferenceObjectiveGVKs))
	for _, gvk := range inferenceObjectiveGVKs {
		if gvk.Group == group {
			supported = append(supported, gvk)
		}
	}
	return supported
}

// supportedPoolGVKsForGroup returns static supported pool GVKs so a supported
// group with a missing CRD is reported as infrastructure-not-ready instead of
// being treated as an arbitrary best-effort API lookup.
func supportedPoolGVKsForGroup(group string) []schema.GroupVersionKind {
	supported := make([]schema.GroupVersionKind, 0, len(inferencePoolGVKs))
	for _, gvk := range inferencePoolGVKs {
		if gvk.Group == group {
			supported = append(supported, gvk)
		}
	}
	return supported
}

func shouldCleanupManagedClusterSPIFFEIDs(conditionType string) bool {
	return conditionType == conditionTypeInvalidRef ||
		conditionType == conditionTypeUnsafeSelector ||
		conditionType == conditionTypeRenderFailure ||
		conditionType == conditionTypeConflict
}

func isInfrastructureNotReadyReason(reason string) bool {
	return reason == "InferenceObjectiveCRDMissing" ||
		reason == "InferencePoolCRDMissing" ||
		reason == "ClusterSPIFFEIDCRDMissing"
}
