package identity

import (
	"errors"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestRenderIdentityRejectsUnsafeSelectors(t *testing.T) {
	t.Parallel()

	binding := testBinding(kleymv1alpha1.InferenceIdentityBindingModePoolOnly)
	binding.Spec.WorkloadSelectorTemplates = []string{"k8s:ns:default"}

	_, err := RenderIdentity(binding, testObjective("objective-a"), testPool("pool-a"))
	if err == nil {
		t.Fatalf("expected unsafe selector error, got nil")
	}

	var stateErr *StateError
	if !errors.As(err, &stateErr) {
		t.Fatalf("expected StateError, got %T", err)
	}
	if stateErr.ConditionType != ConditionTypeUnsafeSelector {
		t.Fatalf("condition = %q, want %q", stateErr.ConditionType, ConditionTypeUnsafeSelector)
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

	podSelector, derivedSelectors, err := DeriveSelectorsFromPool(pool)
	if err != nil {
		t.Fatalf("DeriveSelectorsFromPool returned error: %v", err)
	}
	if _, ok := podSelector["matchLabels"].(map[string]any); !ok {
		t.Fatalf("expected flat selector to normalize into matchLabels, got %v", podSelector)
	}

	for _, expectedSelector := range []string{
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:role:prefill",
	} {
		if !slices.Contains(derivedSelectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, derivedSelectors)
		}
	}
}

func TestRenderIdentityPerObjectiveAddsContainerSelector(t *testing.T) {
	t.Parallel()

	binding := testBinding(kleymv1alpha1.InferenceIdentityBindingModePerObjective)

	identity, err := RenderIdentity(binding, testObjective("objective-a"), testPool("pool-a"))
	if err != nil {
		t.Fatalf("RenderIdentity returned error: %v", err)
	}

	if identity.Mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		t.Fatalf("mode = %q, want %q", identity.Mode, kleymv1alpha1.InferenceIdentityBindingModePerObjective)
	}
	if !slices.Contains(identity.Selectors, "k8s:container-name:main") {
		t.Fatalf("expected container selector, selectors: %v", identity.Selectors)
	}
}

func TestRenderIdentityUsesCustomSPIFFEIDTemplateOverride(t *testing.T) {
	t.Parallel()

	customTemplate := "spiffe://example.test/ns/{{ .Namespace }}/objective/{{ .ObjectiveName }}"
	binding := testBinding(kleymv1alpha1.InferenceIdentityBindingModePerObjective)
	binding.Spec.SpiffeIDTemplate = &customTemplate

	identity, err := RenderIdentity(binding, testObjective("objective-a"), testPool("pool-a"))
	if err != nil {
		t.Fatalf("RenderIdentity returned error: %v", err)
	}

	expectedSPIFFEID := "spiffe://example.test/ns/default/objective/objective-a"
	if identity.SpiffeID != expectedSPIFFEID {
		t.Fatalf("spiffeID = %q, want %q", identity.SpiffeID, expectedSPIFFEID)
	}
}

func TestDesiredClusterSPIFFEIDIncludesHintAndFallback(t *testing.T) {
	t.Parallel()

	binding := testBinding(kleymv1alpha1.InferenceIdentityBindingModePoolOnly)
	identity, err := RenderIdentity(binding, testObjective("objective-a"), testPool("pool-a"))
	if err != nil {
		t.Fatalf("RenderIdentity returned error: %v", err)
	}

	desired := DesiredClusterSPIFFEID(binding, identity)
	spec, found, err := unstructured.NestedMap(desired.Object, "spec")
	if err != nil {
		t.Fatalf("failed to inspect desired ClusterSPIFFEID spec: %v", err)
	}
	if !found {
		t.Fatal("desired ClusterSPIFFEID spec missing")
	}
	if spec["hint"] != "default/binding-a" {
		t.Fatalf("hint = %q, want %q", spec["hint"], "default/binding-a")
	}
	if spec["fallback"] != false {
		t.Fatalf("fallback = %v, want false", spec["fallback"])
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

	_, err := ExtractPoolRef(objective, "default")
	if err == nil {
		t.Fatalf("expected cross-namespace error, got nil")
	}
}

func TestPerObjectiveCollisionFingerprintValidatesInputs(t *testing.T) {
	t.Parallel()

	_, err := PerObjectiveCollisionFingerprint(RenderedIdentity{}, nil)
	if err == nil {
		t.Fatalf("expected nil discriminator error, got nil")
	}

	discriminator := &kleymv1alpha1.ContainerDiscriminator{
		Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
		Value: "main",
	}
	fingerprint, err := PerObjectiveCollisionFingerprint(RenderedIdentity{
		PodSelector: map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:   []string{"k8s:container-name:main", "k8s:ns:default", "k8s:sa:inference-sa"},
	}, discriminator)
	if err != nil {
		t.Fatalf("PerObjectiveCollisionFingerprint returned error: %v", err)
	}
	expected := `{"matchLabels":{"app":"model-server"}}|["k8s:container-name:main","k8s:ns:default","k8s:sa:inference-sa"]|ContainerName|main`
	if fingerprint != expected {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, expected)
	}
}

func testBinding(mode kleymv1alpha1.InferenceIdentityBindingMode) *kleymv1alpha1.InferenceIdentityBinding {
	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-a",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef: kleymv1alpha1.InferencePoolTargetRef{Name: "pool-a"},
			WorkloadSelectorTemplates: []string{
				"k8s:ns:default",
				"k8s:sa:inference-sa",
			},
			Mode: mode,
		},
	}
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		binding.Spec.ObjectiveRef = &kleymv1alpha1.InferenceObjectiveTargetRef{Name: "objective-a"}
		binding.Spec.ContainerDiscriminator = &kleymv1alpha1.ContainerDiscriminator{
			Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
			Value: "main",
		}
	}
	return binding
}

func testPool(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": name},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}
}

func testObjective(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": name},
		},
	}
}
