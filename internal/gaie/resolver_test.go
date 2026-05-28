package gaie

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveInferencePoolUsesAvailableGVKs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, gvk := range InferencePoolGVKs() {
		registerUnstructuredGVK(scheme, gvk)
	}

	preferredPool := testPool("pool-a")
	preferredPool.SetGroupVersionKind(InferencePoolGVKs()[0])
	preferredPool.SetNamespace("default")
	compatiblePool := testPool("pool-a")
	compatiblePool.SetGroupVersionKind(InferencePoolGVKs()[1])
	compatiblePool.SetNamespace("default")
	compatiblePool.Object["spec"] = map[string]any{
		"selector": map[string]any{
			"matchLabels": map[string]any{
				"app": "compatible",
			},
		},
	}

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(preferredPool, compatiblePool).
		Build()

	pool, err := ResolveInferencePool(ctx, reader, []schema.GroupVersionKind{InferencePoolGVKs()[1]}, PoolRef{
		Namespace: "default",
		Name:      "pool-a",
	})
	if err != nil {
		t.Fatalf("ResolveInferencePool returned error: %v", err)
	}
	if pool.GroupVersionKind() != InferencePoolGVKs()[1] {
		t.Fatalf("resolved GVK = %v, want %v", pool.GroupVersionKind(), InferencePoolGVKs()[1])
	}
}

func TestResolveInferenceObjectiveFallsBackAcrossSupportedGVKs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, gvk := range InferenceObjectiveGVKs() {
		registerUnstructuredGVK(scheme, gvk)
	}

	objective := testObjective("objective-a")
	objective.SetNamespace("default")
	objective.SetGroupVersionKind(InferenceObjectiveGVKs()[1])

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objective).
		Build()

	resolved, err := ResolveInferenceObjective(ctx, reader, nil, ObjectiveRef{
		Namespace: "default",
		Name:      "objective-a",
	})
	if err != nil {
		t.Fatalf("ResolveInferenceObjective returned error: %v", err)
	}
	if resolved.GroupVersionKind() != InferenceObjectiveGVKs()[1] {
		t.Fatalf("resolved GVK = %v, want %v", resolved.GroupVersionKind(), InferenceObjectiveGVKs()[1])
	}
}

func TestResolveInferencePoolReportsNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, gvk := range InferencePoolGVKs() {
		registerUnstructuredGVK(scheme, gvk)
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).Build()
	_, err := ResolveInferencePool(ctx, reader, nil, PoolRef{
		Namespace: "default",
		Name:      "missing-pool",
	})
	assertStateError(t, err, ConditionTypeInvalidRef, "TargetPoolNotFound")
}

func TestResolveInferenceObjectiveReportsMissingCRD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gvk := InferenceObjectiveGVKs()[0]
	reader := fixedGetErrorReader{
		err: &meta.NoKindMatchError{
			GroupKind:        gvk.GroupKind(),
			SearchedVersions: []string{gvk.Version},
		},
	}

	_, err := ResolveInferenceObjective(ctx, reader, []schema.GroupVersionKind{gvk}, ObjectiveRef{
		Namespace: "default",
		Name:      "objective-a",
	})
	assertStateError(t, err, ConditionTypeInvalidRef, "InferenceObjectiveCRDMissing")
}

func TestResolveInferencePoolPropagatesUnexpectedReaderError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	readerErr := errors.New("reader unavailable")
	reader := fixedGetErrorReader{err: readerErr}

	_, err := ResolveInferencePool(ctx, reader, []schema.GroupVersionKind{InferencePoolGVKs()[0]}, PoolRef{
		Namespace: "default",
		Name:      "pool-a",
	})
	if !errors.Is(err, readerErr) {
		t.Fatalf("ResolveInferencePool error = %v, want %v", err, readerErr)
	}
}

func TestExtractPoolRefValidatesFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		poolRef map[string]any
		wantErr string
	}{
		{
			name: "missing name",
			poolRef: map[string]any{
				"group": "inference.networking.k8s.io",
			},
			wantErr: "name is required",
		},
		{
			name: "non string group",
			poolRef: map[string]any{
				"name":  "pool-a",
				"group": true,
			},
			wantErr: "group must be a string",
		},
		{
			name: "unsupported group",
			poolRef: map[string]any{
				"name":  "pool-a",
				"group": "unsupported.example.com",
			},
			wantErr: "not a supported GAIE InferencePool group",
		},
		{
			name: "non string namespace",
			poolRef: map[string]any{
				"name":      "pool-a",
				"namespace": true,
			},
			wantErr: "namespace must be a string",
		},
		{
			name: "non string kind",
			poolRef: map[string]any{
				"name": "pool-a",
				"kind": true,
			},
			wantErr: "kind must be a string",
		},
		{
			name: "unsupported kind",
			poolRef: map[string]any{
				"name": "pool-a",
				"kind": "Service",
			},
			wantErr: "kind must be InferencePool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objective := objectiveWithPoolRef(tt.poolRef)
			_, err := ExtractPoolRef(objective, "default")
			if err == nil {
				t.Fatal("ExtractPoolRef returned nil error, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ExtractPoolRef error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExtractPoolRefNormalizesDefaultNamespaceAndGroup(t *testing.T) {
	t.Parallel()

	objective := objectiveWithPoolRef(map[string]any{
		"name":      " pool-a ",
		"group":     " inference.networking.k8s.io ",
		"namespace": "default",
		"kind":      "InferencePool",
	})

	ref, err := ExtractPoolRef(objective, "default")
	if err != nil {
		t.Fatalf("ExtractPoolRef returned error: %v", err)
	}
	if ref != (PoolRef{Name: "pool-a", Group: "inference.networking.k8s.io", Namespace: "default"}) {
		t.Fatalf("ExtractPoolRef = %+v, want normalized pool ref", ref)
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

func TestDeriveSelectorsFromPoolRejectsInvalidMatchLabels(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		"array-value":               {"app": []any{"model-server"}},
		"bool-value":                {"app": true},
		"number-value":              {"app": float64(1)},
		"object-value":              {"app": map[string]any{"name": "model-server"}},
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

			pool := testPool("pool-invalid-labels")
			pool.Object["spec"] = map[string]any{
				"selector": map[string]any{
					"matchLabels": labels,
				},
			}

			_, _, err := DeriveSelectorsFromPool(pool)
			if err == nil {
				t.Fatalf("expected invalid matchLabels error, got nil")
			}
			if !strings.Contains(err.Error(), "pool spec.selector.matchLabels") &&
				!strings.Contains(err.Error(), "pool selector labels") {
				t.Fatalf("error = %q, want matchLabels validation error", err.Error())
			}
		})
	}
}

func TestValidateObjectiveTargetsPool(t *testing.T) {
	t.Parallel()

	objective := objectiveWithPoolRef(map[string]any{
		"name":  "pool-a",
		"group": "inference.networking.k8s.io",
	})
	objective.SetName("objective-a")
	objective.SetNamespace("default")

	pool := testPool("pool-a")
	pool.SetNamespace("default")
	pool.SetGroupVersionKind(InferencePoolGVKs()[0])

	if err := ValidateObjectiveTargetsPool(objective, pool, "default"); err != nil {
		t.Fatalf("ValidateObjectiveTargetsPool returned error: %v", err)
	}

	pool.SetName("pool-b")
	if err := ValidateObjectiveTargetsPool(objective, pool, "default"); err == nil {
		t.Fatal("expected pool name mismatch error, got nil")
	}

	pool.SetName("pool-a")
	pool.SetGroupVersionKind(InferencePoolGVKs()[1])
	if err := ValidateObjectiveTargetsPool(objective, pool, "default"); err == nil {
		t.Fatal("expected pool group mismatch error, got nil")
	}
}

func registerUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}

func objectiveWithPoolRef(poolRef map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "objective-a"},
			"spec": map[string]any{
				"poolRef": poolRef,
			},
		},
	}
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

func assertStateError(t *testing.T, err error, conditionType, reason string) {
	t.Helper()

	var stateErr *StateError
	if !errors.As(err, &stateErr) {
		t.Fatalf("error = %T %v, want StateError", err, err)
	}
	if stateErr.ConditionType != conditionType || stateErr.Reason != reason {
		t.Fatalf(
			"StateError = condition %q reason %q, want condition %q reason %q",
			stateErr.ConditionType,
			stateErr.Reason,
			conditionType,
			reason,
		)
	}
}

type fixedGetErrorReader struct {
	err error
}

func (r fixedGetErrorReader) Get(
	_ context.Context,
	_ types.NamespacedName,
	_ client.Object,
	_ ...client.GetOption,
) error {
	return r.err
}

func (r fixedGetErrorReader) List(
	_ context.Context,
	_ client.ObjectList,
	_ ...client.ListOption,
) error {
	return apierrors.NewNotFound(schema.GroupResource{Group: "test.example.com", Resource: "lists"}, "unused")
}

var _ client.Reader = fixedGetErrorReader{}
