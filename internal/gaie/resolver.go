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

	"github.com/sonda-red/kleym/internal/identity"
)

const inferencePoolIdentityAnchorKind = "pool"

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
			ReasonInferencePoolCRDMissing,
			"InferencePool CRD is not installed",
		)
	}
	return nil, newStateError(
		ConditionTypeInvalidRef,
		ReasonTargetPoolNotFound,
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

// DeriveSelectorsFromPool extracts deterministic pod-level selectors from an InferencePool.
func DeriveSelectorsFromPool(pool *unstructured.Unstructured) (map[string]any, []string, error) {
	selectorMap, found, err := unstructured.NestedMap(pool.Object, "spec", "selector")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode pool spec.selector: %w", err)
	}
	if !found || len(selectorMap) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector must be set")
	}

	if _, hasExpressions := selectorMap["matchExpressions"]; hasExpressions {
		return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions are not supported")
	}

	var matchLabels map[string]any
	if rawMatchLabels, hasMatchLabels := selectorMap["matchLabels"]; hasMatchLabels {
		for key := range selectorMap {
			if key != "matchLabels" {
				return nil, nil, fmt.Errorf("pool spec.selector.%s is not supported", key)
			}
		}
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

// ResolveInferenceTarget maps a resolved GAIE InferencePool to source-independent identity inputs.
func ResolveInferenceTarget(pool *unstructured.Unstructured) (identity.ResolvedInferenceTarget, error) {
	podSelector, derivedSelectors, err := DeriveSelectorsFromPool(pool)
	if err != nil {
		return identity.ResolvedInferenceTarget{}, err
	}

	return identity.ResolvedInferenceTarget{
		IdentityAnchor: identity.IdentityAnchor{
			Kind: inferencePoolIdentityAnchorKind,
			Name: pool.GetName(),
		},
		PodSelector:      podSelector,
		DerivedSelectors: derivedSelectors,
	}, nil
}
