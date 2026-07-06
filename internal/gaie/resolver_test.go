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
	currentPoolGVK := InferencePoolGVKs()[0]
	registerUnstructuredGVK(scheme, currentPoolGVK)

	poolObject := testPool("pool-a")
	poolObject.SetGroupVersionKind(currentPoolGVK)
	poolObject.SetNamespace("default")

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(poolObject).
		Build()

	pool, err := ResolveInferencePool(ctx, reader, []schema.GroupVersionKind{currentPoolGVK}, PoolRef{
		Namespace: "default",
		Name:      "pool-a",
	})
	if err != nil {
		t.Fatalf("ResolveInferencePool returned error: %v", err)
	}
	if pool.GroupVersionKind() != currentPoolGVK {
		t.Fatalf("resolved GVK = %v, want %v", pool.GroupVersionKind(), currentPoolGVK)
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

func TestResolveInferencePoolReportsMissingCRD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gvk := InferencePoolGVKs()[0]
	reader := fixedGetErrorReader{
		err: &meta.NoKindMatchError{
			GroupKind:        gvk.GroupKind(),
			SearchedVersions: []string{gvk.Version},
		},
	}

	_, err := ResolveInferencePool(ctx, reader, []schema.GroupVersionKind{gvk}, PoolRef{
		Namespace: "default",
		Name:      "pool-a",
	})
	assertStateError(t, err, ConditionTypeInvalidRef, "InferencePoolCRDMissing")
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

func registerUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
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
