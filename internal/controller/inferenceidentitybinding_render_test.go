package controller

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestRenderIdentityRejectsUnsafeSelectors(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-a",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-a"},
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
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
		t.Fatalf("expected unsafe selector error, got nil")
	}

	var stateErr reconcileStateError
	if !errorsAsStateError(err, &stateErr) {
		t.Fatalf("expected reconcileStateError, got %T", err)
	}
	if stateErr.conditionType != conditionTypeUnsafeSelector {
		t.Fatalf("expected condition %q, got %q", conditionTypeUnsafeSelector, stateErr.conditionType)
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
					TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-non-string-label"},
					WorkloadSelectorTemplates: []string{
						"k8s:ns:default",
						"k8s:sa:inference-sa",
					},
					Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
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

func TestRenderIdentityRendersStringPoolMatchLabels(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-string-labels",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-string-labels"},
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
				"k8s:sa:inference-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
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
						"app":  "model-server",
						"role": "decode",
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
		"k8s:pod-label:role:decode",
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

	podSelector, derivedSelectors, err := deriveSelectorsFromPool(pool)
	if err != nil {
		t.Fatalf("deriveSelectorsFromPool returned error: %v", err)
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
			TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-b"},
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
				"k8s:sa:inference-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerDiscriminator: &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "main",
			},
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

func TestRenderIdentityUsesCustomSPIFFEIDTemplateOverride(t *testing.T) {
	t.Parallel()

	reconciler := &InferenceIdentityBindingReconciler{}
	customTemplate := "spiffe://example.test/ns/{{ .Namespace }}/objective/{{ .ObjectiveName }}"
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-custom-template",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef:        kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-custom"},
			SpiffeIDTemplate: &customTemplate,
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
				"k8s:sa:inference-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerDiscriminator: &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "main",
			},
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

	expectedSPIFFEID := "spiffe://example.test/ns/default/objective/objective-custom"
	if identity.SpiffeID != expectedSPIFFEID {
		t.Fatalf("rendered spiffeID = %q, want %q", identity.SpiffeID, expectedSPIFFEID)
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

	_, err := extractPoolRef(objective, "default")
	if err == nil {
		t.Fatalf("expected cross-namespace error, got nil")
	}
}
