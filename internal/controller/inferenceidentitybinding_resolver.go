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
)

func (r *InferenceIdentityBindingReconciler) resolveInferenceObjective(
	ctx context.Context,
	namespace string,
	name string,
) (*unstructured.Unstructured, error) {
	objective, crdMissing, err := r.resolveByCandidates(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: name},
		r.resolveObjectiveGVKs(),
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
		fmt.Sprintf("targetRef %q was not found", name),
	)
}

func (r *InferenceIdentityBindingReconciler) resolveInferencePool(
	ctx context.Context,
	poolRef inferencePoolRef,
) (*unstructured.Unstructured, error) {
	poolCandidates := candidatePoolGVKs(r.resolvePoolGVKs(), poolRef.Group)
	if poolRef.Group != "" && len(poolCandidates) == 0 {
		return nil, newStateError(
			conditionTypeInvalidRef,
			"UnsupportedPoolGroup",
			fmt.Sprintf("poolRef.group %q is not a supported GAIE InferencePool group", poolRef.Group),
		)
	}

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

func candidatePoolGVKs(candidates []schema.GroupVersionKind, group string) []schema.GroupVersionKind {
	if group == "" {
		return candidates
	}

	if !isSupportedPoolGroup(group) {
		return nil
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

// isSupportedPoolGroup keeps poolRef.group constrained to the GAIE groups
// listed in docs/spec.md instead of probing arbitrary API groups.
func isSupportedPoolGroup(group string) bool {
	for _, gvk := range inferencePoolGVKs {
		if gvk.Group == group {
			return true
		}
	}
	return false
}

// supportedPoolGVKsForGroup returns only documented InferencePool GVKs for a
// supported group, preserving the compatibility matrix in docs/spec.md.
func supportedPoolGVKsForGroup(group string) []schema.GroupVersionKind {
	gvks := make([]schema.GroupVersionKind, 0, len(inferencePoolGVKs))
	for _, gvk := range inferencePoolGVKs {
		if gvk.Group == group {
			gvks = append(gvks, gvk)
		}
	}
	return gvks
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
