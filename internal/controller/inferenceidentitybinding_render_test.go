package controller

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestRenderIdentityRejectsInvalidServiceAccountName(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-a",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-a"},
			ServiceAccountName: "Invalid_ServiceAccount",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-a"},
		},
	}
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-a"},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}

	_, err := reconciler.renderIdentity(binding, objective, pool)
	if err == nil {
		t.Fatalf("expected invalid service account error, got nil")
	}

	var stateErr reconcileStateError
	if !errorsAsStateError(err, &stateErr) {
		t.Fatalf("expected reconcileStateError, got %T", err)
	}
	if stateErr.conditionType != conditionTypeRenderFailure || stateErr.reason != "InvalidServiceAccountName" {
		t.Fatalf("expected condition/reason %q/InvalidServiceAccountName, got %q/%q", conditionTypeRenderFailure, stateErr.conditionType, stateErr.reason)
	}
}

func TestRenderIdentityRejectsNonStringPoolMatchLabelValues(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		"array":  []any{"model-server"},
		"bool":   true,
		"number": float64(1),
		"object": map[string]any{"name": "model-server"},
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reconciler := &InferenceIdentityBindingReconciler{}
			binding := &kleymv1alpha1.InferenceIdentityBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding-non-string-label",
					Namespace: "default",
				},
				Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
					PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-non-string-label"},
					ServiceAccountName: "inference-sa",
					Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
				},
			}
			objective := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{"name": "objective-non-string-label"},
				},
			}
			pool := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{"name": "pool-non-string-label"},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": value,
							},
						},
					},
				},
			}

			_, err := reconciler.renderIdentity(binding, objective, pool)
			if err == nil {
				t.Fatalf("expected invalid pool selector error, got nil")
			}

			var stateErr reconcileStateError
			if !errorsAsStateError(err, &stateErr) {
				t.Fatalf("expected reconcileStateError, got %T", err)
			}
			if stateErr.conditionType != conditionTypeUnsafeSelector {
				t.Fatalf("expected condition %q, got %q", conditionTypeUnsafeSelector, stateErr.conditionType)
			}
			if stateErr.reason != "InvalidPoolSelector" {
				t.Fatalf("expected reason %q, got %q", "InvalidPoolSelector", stateErr.reason)
			}
		})
	}
}

func TestRenderIdentityRejectsInvalidPoolMatchLabelSyntax(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		"invalid-key-prefix":        {"Example.com/app": "model-server"},
		"invalid-key-name":          {"app/name/extra": "model-server"},
		"leading-whitespace-key":    {" app": "model-server"},
		"leading-whitespace-value":  {"app": " model-server"},
		"trailing-whitespace-value": {"app": "model-server "},
		"whitespace-only-value":     {"app": " "},
		"invalid-value-character":   {"app": "model/server"},
		"invalid-value-start":       {"app": "-model"},
		"invalid-value-end":         {"app": "model-"},
	}

	for name, labels := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reconciler := &InferenceIdentityBindingReconciler{}
			binding := &kleymv1alpha1.InferenceIdentityBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding-invalid-label-syntax",
					Namespace: "default",
				},
				Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
					PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-invalid-label-syntax"},
					ServiceAccountName: "inference-sa",
					Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
				},
			}
			objective := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{"name": "objective-invalid-label-syntax"},
				},
			}
			pool := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{"name": "pool-invalid-label-syntax"},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": labels,
						},
					},
				},
			}

			_, err := reconciler.renderIdentity(binding, objective, pool)
			if err == nil {
				t.Fatalf("expected invalid pool selector error, got nil")
			}

			var stateErr reconcileStateError
			if !errorsAsStateError(err, &stateErr) {
				t.Fatalf("expected reconcileStateError, got %T", err)
			}
			if stateErr.conditionType != conditionTypeUnsafeSelector {
				t.Fatalf("expected condition %q, got %q", conditionTypeUnsafeSelector, stateErr.conditionType)
			}
			if stateErr.reason != "InvalidPoolSelector" {
				t.Fatalf("expected reason %q, got %q", "InvalidPoolSelector", stateErr.reason)
			}
		})
	}
}

func TestRenderIdentityRendersStringPoolMatchLabels(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-string-labels",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-string-labels"},
			ServiceAccountName: "inference-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-string-labels"},
		},
	}
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-string-labels"},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app":                                "model-server",
						"inference.networking.x-k8s.io/role": "decode.v1",
					},
				},
			},
		},
	}

	identity, err := reconciler.renderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	expectedSelectors := []string{
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:inference.networking.x-k8s.io/role:decode.v1",
	}
	for _, expectedSelector := range expectedSelectors {
		if !slices.Contains(identity.Selectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, identity.Selectors)
		}
	}
}

func TestDeriveSelectorsFromPoolKeepsFlatStringMapCompatibility(t *testing.T) {
	t.Parallel()

	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-flat-selector"},
			"spec": map[string]any{
				"selector": map[string]any{
					"app":  "model-server",
					"role": "prefill",
				},
			},
		},
	}

	podSelector, derivedSelectors, err := gaie.DeriveSelectorsFromPool(pool)
	if err != nil {
		t.Fatalf("DeriveSelectorsFromPool returned error: %v", err)
	}

	if _, ok := podSelector["matchLabels"].(map[string]any); !ok {
		t.Fatalf("expected flat selector to normalize into matchLabels, got %v", podSelector)
	}

	expectedSelectors := []string{
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:role:prefill",
	}
	for _, expectedSelector := range expectedSelectors {
		if !slices.Contains(derivedSelectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, derivedSelectors)
		}
	}
}

func TestRenderIdentityPerObjectiveAddsContainerSelector(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-b",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-b"},
			ObjectiveRef:       &kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-b"},
			ServiceAccountName: "inference-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerName:      "main",
		},
	}
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-b"},
		},
	}
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-b"},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}

	identity, err := reconciler.renderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	if identity.Mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		t.Fatalf("expected mode %q, got %q", kleymv1alpha1.InferenceIdentityBindingModePerObjective, identity.Mode)
	}

	foundContainerSelector := false
	for _, selector := range identity.Selectors {
		if selector == "k8s:container-name:main" {
			foundContainerSelector = true
			break
		}
	}
	if !foundContainerSelector {
		t.Fatalf("expected container selector to be rendered, selectors: %v", identity.Selectors)
	}
}

func TestRenderIdentityUsesDeterministicSPIFFEID(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-custom-template",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-custom"},
			ObjectiveRef:       &kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-custom"},
			ServiceAccountName: "inference-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerName:      "main",
		},
	}
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-custom"},
		},
	}
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-custom"},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}

	identity, err := reconciler.renderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}

	expectedSPIFFEID := "spiffe://kleym.sonda.red/ns/default/objective/objective-custom"
	if identity.SpiffeID != expectedSPIFFEID {
		t.Fatalf("rendered spiffeID = %q, want %q", identity.SpiffeID, expectedSPIFFEID)
	}
}

func TestRenderIdentityIncludesHintAndFallback(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-hint-fallback",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-hint"},
			ServiceAccountName: "inference-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-hint"},
		},
	}
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "pool-hint"},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}

	identity, err := reconciler.renderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("renderIdentity returned error: %v", err)
	}
	desired := spirecm.DesiredClusterSPIFFEID(binding, identity)

	spec, found, err := unstructured.NestedMap(desired.Object, "spec")
	if err != nil {
		t.Fatalf("failed to inspect desired ClusterSPIFFEID spec: %v", err)
	}
	if !found {
		t.Fatal("desired ClusterSPIFFEID spec missing")
	}

	hint, ok := spec["hint"].(string)
	if !ok {
		t.Fatalf("expected spec.hint to be string, got %T", spec["hint"])
	}
	if hint != "default/binding-hint-fallback" {
		t.Fatalf("spec.hint = %q, want %q", hint, "default/binding-hint-fallback")
	}

	fallback, ok := spec["fallback"].(bool)
	if !ok {
		t.Fatalf("expected spec.fallback to be bool, got %T", spec["fallback"])
	}
	if fallback != false {
		t.Fatalf("spec.fallback = %v, want false", fallback)
	}
}

func TestExtractPoolRefRejectsCrossNamespace(t *testing.T) {
	t.Parallel()

	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"poolRef": map[string]any{
					"name":      "pool-a",
					"namespace": "other",
				},
			},
		},
	}

	_, err := gaie.ExtractPoolRef(objective, "default")
	if err == nil {
		t.Fatalf("expected cross-namespace error, got nil")
	}
}
