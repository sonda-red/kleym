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
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveInferenceObjective reads an objective from the first matching supported GAIE GVK.
func ResolveInferenceObjective(
	ctx context.Context,
	reader client.Reader,
	availableObjectiveGVKs []schema.GroupVersionKind,
	objectiveRef ObjectiveRef,
) (*unstructured.Unstructured, error) {
	objectiveCandidates := CandidateObjectiveGVKs(ResolveObjectiveGVKs(availableObjectiveGVKs), objectiveRef.Group)
	objective, crdMissing, err := resolveByCandidates(
		ctx,
		reader,
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
			ConditionTypeInvalidRef,
			"InferenceObjectiveCRDMissing",
			"InferenceObjective CRD is not installed",
		)
	}
	return nil, newStateError(
		ConditionTypeInvalidRef,
		"TargetObjectiveNotFound",
		fmt.Sprintf("objectiveRef %q was not found", objectiveRef.Name),
	)
}

// ResolveInferencePool reads a pool from the first matching supported GAIE GVK.
func ResolveInferencePool(
	ctx context.Context,
	reader client.Reader,
	availablePoolGVKs []schema.GroupVersionKind,
	poolRef PoolRef,
) (*unstructured.Unstructured, error) {
	poolCandidates := CandidatePoolGVKs(ResolvePoolGVKs(availablePoolGVKs), poolRef.Group)
	pool, crdMissing, err := resolveByCandidates(
		ctx,
		reader,
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
			ConditionTypeInvalidRef,
			"InferencePoolCRDMissing",
			"InferencePool CRD is not installed",
		)
	}
	return nil, newStateError(
		ConditionTypeInvalidRef,
		"TargetPoolNotFound",
		fmt.Sprintf("poolRef %q was not found", poolRef.Name),
	)
}

func resolveByCandidates(
	ctx context.Context,
	reader client.Reader,
	key types.NamespacedName,
	candidates []schema.GroupVersionKind,
) (*unstructured.Unstructured, bool, error) {
	crdMissing := false

	for _, gvk := range candidates {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		err := reader.Get(ctx, key, obj)
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

// ExtractPoolRef reads an objective's poolRef using GAIE-compatible unstructured fields.
func ExtractPoolRef(objective *unstructured.Unstructured, defaultNamespace string) (PoolRef, error) {
	poolRefMap, found, err := unstructured.NestedMap(objective.Object, "spec", "poolRef")
	if err != nil {
		return PoolRef{}, fmt.Errorf("failed to decode objective spec.poolRef: %w", err)
	}
	if !found {
		return PoolRef{}, fmt.Errorf("objective spec.poolRef is required")
	}

	name, ok := poolRefMap["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return PoolRef{}, fmt.Errorf("objective spec.poolRef.name is required")
	}

	group := ""
	if rawGroup, exists := poolRefMap["group"]; exists {
		groupValue, groupOK := rawGroup.(string)
		if !groupOK {
			return PoolRef{}, fmt.Errorf("objective spec.poolRef.group must be a string")
		}
		group = strings.TrimSpace(groupValue)
		if group != "" && !IsSupportedInferencePoolGroup(group) {
			return PoolRef{}, fmt.Errorf("objective spec.poolRef.group %q is not a supported GAIE InferencePool group", group)
		}
	}

	namespace := defaultNamespace
	if rawNamespace, exists := poolRefMap["namespace"]; exists {
		namespaceValue, namespaceOK := rawNamespace.(string)
		if !namespaceOK {
			return PoolRef{}, fmt.Errorf("objective spec.poolRef.namespace must be a string")
		}
		if namespaceValue != "" && namespaceValue != defaultNamespace {
			return PoolRef{}, fmt.Errorf("cross-namespace poolRef is not allowed")
		}
		if namespaceValue != "" {
			namespace = namespaceValue
		}
	}

	if rawKind, exists := poolRefMap["kind"]; exists {
		kindValue, kindOK := rawKind.(string)
		if !kindOK {
			return PoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be a string")
		}
		if kindValue != "" && kindValue != "InferencePool" {
			return PoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be InferencePool")
		}
	}

	return PoolRef{
		Name:      strings.TrimSpace(name),
		Group:     group,
		Namespace: namespace,
	}, nil
}

// ValidateObjectiveTargetsPool preserves the pool-first selector provenance contract.
func ValidateObjectiveTargetsPool(
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
	defaultNamespace string,
) error {
	objectivePoolRef, err := ExtractPoolRef(objective, defaultNamespace)
	if err != nil {
		return err
	}

	if objectivePoolRef.Name != pool.GetName() || objectivePoolRef.Namespace != pool.GetNamespace() {
		return fmt.Errorf(
			"objectiveRef %q points at poolRef %q, want %q",
			objective.GetName(),
			namespacedKey(objectivePoolRef.Namespace, objectivePoolRef.Name),
			namespacedKey(pool.GetNamespace(), pool.GetName()),
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

// DeriveSelectorsFromPool extracts deterministic pod-level selectors from an InferencePool.
func DeriveSelectorsFromPool(pool *unstructured.Unstructured) (map[string]any, []string, error) {
	selectorMap, found, err := unstructured.NestedMap(pool.Object, "spec", "selector")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode pool spec.selector: %w", err)
	}
	if !found || len(selectorMap) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector must be set")
	}

	var matchLabels map[string]any
	if rawMatchLabels, hasMatchLabels := selectorMap["matchLabels"]; hasMatchLabels {
		typedMatchLabels, ok := rawMatchLabels.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must be an object")
		}
		matchLabels = typedMatchLabels
	} else {
		isFlatSelector := true
		for _, value := range selectorMap {
			if _, ok := value.(string); !ok {
				isFlatSelector = false
				break
			}
		}
		if !isFlatSelector {
			return nil, nil, fmt.Errorf("pool selector must use matchLabels for deterministic rendering")
		}
		matchLabels = selectorMap
		selectorMap = map[string]any{"matchLabels": matchLabels}
	}

	if rawExpressions, hasExpressions := selectorMap["matchExpressions"]; hasExpressions {
		expressions, ok := rawExpressions.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions must be an array")
		}
		if len(expressions) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions are not supported")
		}
	}

	if len(matchLabels) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must not be empty")
	}

	derivedSelectors := make([]string, 0, len(matchLabels))
	for key, value := range matchLabels {
		valueText, ok := value.(string)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels[%q] must be a string", key)
		}
		if key == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty keys")
		}
		if valueText == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty values")
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels key %q is invalid: %s", key, strings.Join(errs, "; "))
		}
		if errs := validation.IsValidLabelValue(valueText); len(errs) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels[%q] value %q is invalid: %s", key, valueText, strings.Join(errs, "; "))
		}
		derivedSelectors = append(derivedSelectors, fmt.Sprintf("k8s:pod-label:%s:%s", key, valueText))
	}

	return selectorMap, derivedSelectors, nil
}

func namespacedKey(namespace, name string) string {
	return types.NamespacedName{Namespace: namespace, Name: name}.String()
}
