package identity

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func registerUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}
