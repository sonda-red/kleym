package controller

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestRenderIdentityMapsInvalidPoolSelectorToUnsafeSelector(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := testRenderBinding("binding-invalid-selector", "pool-invalid-selector")
	pool := testRenderPool("pool-invalid-selector", map[string]any{
		"app": []any{"model-server"},
	})

	_, err := reconciler.renderIdentity(binding, nil, pool)
	if err == nil {
		t.Fatalf("expected invalid pool selector error, got nil")
	}

	var stateErr reconcileStateError
	if !errorsAsStateError(err, &stateErr) {
		t.Fatalf("expected reconcileStateError, got %T", err)
	}
	if stateErr.conditionType != conditionTypeUnsafeSelector {
		t.Fatalf("conditionType = %q, want %q", stateErr.conditionType, conditionTypeUnsafeSelector)
	}
	if stateErr.reason != "InvalidPoolSelector" {
		t.Fatalf("reason = %q, want InvalidPoolSelector", stateErr.reason)
	}
}

func TestRenderIdentityPassesValidGAIESelectorIntoIdentityPlan(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := testRenderBinding("binding-valid-selector", "pool-valid-selector")
	pool := testRenderPool("pool-valid-selector", map[string]any{
		"app":                                "model-server",
		"inference.networking.x-k8s.io/role": "decode.v1",
	})

	identity, err := reconciler.renderIdentity(binding, nil, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	for _, expectedSelector := range []string{
		"k8s:ns:default",
		"k8s:sa:inference-sa",
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:inference.networking.x-k8s.io/role:decode.v1",
	} {
		if !slices.Contains(identity.Selectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, identity.Selectors)
		}
	}
	if identity.PodSelector["matchLabels"] == nil {
		t.Fatalf("expected GAIE matchLabels selector to be passed into identity plan, got %v", identity.PodSelector)
	}
}

func testRenderBinding(name, poolName string) *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: poolName},
			ServiceAccountName: "inference-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
}

func testRenderPool(name string, matchLabels map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": name},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": matchLabels,
				},
			},
		},
	}
}
