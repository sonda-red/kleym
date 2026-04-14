package controller

import (
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

	stateErr, ok := err.(*reconcileStateError)
	if !ok {
		t.Fatalf("expected reconcileStateError, got %T", err)
	}
	if stateErr.conditionType != conditionTypeUnsafeSelector {
		t.Fatalf("expected condition %q, got %q", conditionTypeUnsafeSelector, stateErr.conditionType)
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
