package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const testNamespace = "default"

func newCollisionTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kleymv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add kleym scheme: %v", err)
	}
	registerUnstructuredGVK(scheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferencePoolGVKs {
		registerUnstructuredGVK(scheme, gvk)
	}

	return scheme
}

func registerUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}

func newTestPool() *unstructured.Unstructured {
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}
	pool.SetGroupVersionKind(inferencePoolGVKs[0])
	pool.SetNamespace(testNamespace)
	pool.SetName("pool-a")
	return pool
}

func newPoolOnlyBinding(name, _ string) *kleymv1alpha1.InferenceIdentityBinding {
	return newPoolBindingWithServiceAccount(name, "inference-sa")
}

func newPoolBindingWithServiceAccount(
	name string,
	serviceAccount string,
) *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef: kleymv1alpha1.InferencePoolTargetRef{
				Name: "pool-a",
			},
			ServiceAccountName: serviceAccount,
		},
	}
}

func assertConditionStatus(
	t *testing.T,
	ctx context.Context,
	cli client.Client,
	name string,
	conditionType string,
	expectedStatus metav1.ConditionStatus,
	expectedReason string,
) {
	t.Helper()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, binding); err != nil {
		t.Fatalf("failed to fetch %s/%s: %v", testNamespace, name, err)
	}
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionType)
	if condition == nil {
		t.Fatalf("expected condition %q on %s/%s", conditionType, testNamespace, name)
	}
	if condition.Status != expectedStatus {
		t.Fatalf("condition %q on %s/%s status = %q, want %q", conditionType, testNamespace, name, condition.Status, expectedStatus)
	}
	if expectedReason != "" && condition.Reason != expectedReason {
		t.Fatalf("condition %q on %s/%s reason = %q, want %q", conditionType, testNamespace, name, condition.Reason, expectedReason)
	}
}

func assertClusterSPIFFEIDCount(t *testing.T, ctx context.Context, cli client.Client, expected int) {
	t.Helper()

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))
	if err := cli.List(ctx, list); err != nil {
		t.Fatalf("failed to list ClusterSPIFFEIDs: %v", err)
	}
	if len(list.Items) != expected {
		t.Fatalf("ClusterSPIFFEID count = %d, want %d", len(list.Items), expected)
	}
}
