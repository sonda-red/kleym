package controller

import (
	"reflect"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestRenderIdentityMapsInvalidPoolSelectorToUnsafeSelector(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig()}
	binding := testRenderBinding("binding-invalid-selector", "pool-invalid-selector")
	pool := testRenderPool("pool-invalid-selector", map[string]any{
		"app": []any{"model-server"},
	})

	_, err := reconciler.renderIdentity(binding, pool)
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

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig()}
	binding := testRenderBinding("binding-valid-selector", "pool-valid-selector")
	pool := testRenderPool("pool-valid-selector", map[string]any{
		"app":                              "model-server",
		"inference.networking.k8s.io/role": "decode.v1",
	})

	identity, err := reconciler.renderIdentity(binding, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	for _, expectedSelector := range []string{
		"k8s:ns:default",
		"k8s:sa:inference-sa",
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:identity.kleym.sonda.red/variant:prefill",
		"k8s:pod-label:inference.networking.k8s.io/role:decode.v1",
	} {
		if !slices.Contains(identity.Selectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, identity.Selectors)
		}
	}
	wantPodSelector := map[string]any{"matchLabels": map[string]any{
		"app":                              "model-server",
		"inference.networking.k8s.io/role": "decode.v1",
	}}
	if !reflect.DeepEqual(identity.PodSelector, wantPodSelector) {
		t.Fatalf("podSelector = %#v, want preserved normalized pool selector %#v", identity.PodSelector, wantPodSelector)
	}
	if identity.SpiffeID != "spiffe://kleym.sonda.red/ns/default/sa/inference-sa/inference/pool/pool-valid-selector/variant/prefill" {
		t.Fatalf("spiffeID = %q, want service-account-scoped pool target identity", identity.SpiffeID)
	}
}

func TestRenderIdentityDistinguishesServiceAccountsForTheSamePool(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig()}
	pool := testRenderPool("pool-a", map[string]any{"app": "model-server"})

	firstBinding := testRenderBinding("binding-a", "pool-a")
	first, err := reconciler.renderIdentity(firstBinding, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	secondBinding := testRenderBinding("binding-b", "pool-a")
	secondBinding.Spec.ServiceAccountName = "other-inference-sa"
	second, err := reconciler.renderIdentity(secondBinding, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	if first.SpiffeID == second.SpiffeID {
		t.Fatalf("SPIFFE IDs match for different service accounts: %q", first.SpiffeID)
	}

	samePrincipalBinding := testRenderBinding("binding-c", "pool-a")
	samePrincipal, err := reconciler.renderIdentity(samePrincipalBinding, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}
	if first.SpiffeID != samePrincipal.SpiffeID {
		t.Fatalf("binding name changed SPIFFE ID: %q != %q", first.SpiffeID, samePrincipal.SpiffeID)
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
			IdentityBoundary:   testIdentityBoundary,
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
