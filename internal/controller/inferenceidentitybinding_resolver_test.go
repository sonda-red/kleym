package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestReconcileSetsInvalidRefWhenPoolCannotBeResolved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPerObjectiveBinding("binding-missing-pool", "objective-missing-pool")
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(
				newTestObjective("objective-missing-pool"),
				binding,
			).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "TargetPoolNotFound")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "TargetPoolNotFound")
}

func TestReconcilePoolOnlyDoesNotRequireObjective(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPoolOnlyBinding("binding-pool-only", "")
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func TestReconcileUsesOperatorConfigForRenderedOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPoolOnlyBinding("binding-operator-config", "")
	reconciler := &InferenceIdentityBindingReconciler{Config: OperatorConfig{
		TrustDomain:              "example.org",
		ClusterSPIFFEIDClassName: "kleym",
	},
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))
	if err := reconciler.List(ctx, list); err != nil {
		t.Fatalf("failed to list ClusterSPIFFEIDs: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("ClusterSPIFFEID count = %d, want 1", len(list.Items))
	}
	spiffeID, _, err := unstructured.NestedString(list.Items[0].Object, "spec", "spiffeIDTemplate")
	if err != nil {
		t.Fatalf("failed to read spec.spiffeIDTemplate: %v", err)
	}
	if spiffeID != "spiffe://example.org/ns/default/pool/pool-a" {
		t.Fatalf("spiffeIDTemplate = %q, want spiffe://example.org/ns/default/pool/pool-a", spiffeID)
	}
	className, _, err := unstructured.NestedString(list.Items[0].Object, "spec", "className")
	if err != nil {
		t.Fatalf("failed to read spec.className: %v", err)
	}
	if className != "kleym" {
		t.Fatalf("className = %q, want kleym", className)
	}
}

func TestReconcilePerObjectiveRequiresObjectiveRef(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPoolOnlyBinding("binding-missing-objective-ref", "")
	binding.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePerObjective
	binding.Spec.ContainerName = "main"
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeRenderFailure, metav1.ConditionTrue, "MissingObjectiveRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "MissingObjectiveRef")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
}

func TestReconcilePerObjectiveFailsWhenObjectiveRefMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPerObjectiveBinding("binding-missing-objective", "missing-objective")
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "TargetObjectiveNotFound")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "TargetObjectiveNotFound")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
}

func TestReconcileSetsInvalidRefWhenObjectivePointsAtDifferentPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	objective := newTestObjective("objective-mismatch")
	objective.Object["spec"] = map[string]any{
		"poolRef": map[string]any{
			"name": "pool-other",
		},
	}

	binding := newPerObjectiveBinding("binding-objective-pool-mismatch", objective.GetName())
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), objective, binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "InvalidObjectiveRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "InvalidObjectiveRef")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
}

func TestReconcileSetsInvalidRefWhenObjectivePoolRefIsInvalid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"poolRef": map[string]any{
					"group": "inference.networking.k8s.io",
				},
			},
		},
	}
	objective.SetGroupVersionKind(inferenceObjectiveGVKs[0])
	objective.SetNamespace(testNamespace)
	objective.SetName("objective-invalid-pool-ref")

	binding := newPerObjectiveBinding("binding-invalid-pool-ref", objective.GetName())
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), objective, binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "InvalidObjectiveRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "InvalidObjectiveRef")
}

func TestReconcileSetsInvalidRefWhenObjectivePoolRefGroupIsUnsupported(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	unsupportedPoolGVK := schema.GroupVersionKind{
		Group:   "inference.example.com",
		Version: "v1",
		Kind:    "InferencePool",
	}
	registerUnstructuredGVK(scheme, unsupportedPoolGVK)

	pool := newTestPool()
	pool.SetGroupVersionKind(unsupportedPoolGVK)
	pool.SetName("pool-unsupported")

	objective := newTestObjective("objective-unsupported-pool-group")
	objective.Object["spec"] = map[string]any{
		"poolRef": map[string]any{
			"name":  pool.GetName(),
			"group": unsupportedPoolGVK.Group,
		},
	}

	binding := newPerObjectiveBinding("binding-unsupported-pool-group", objective.GetName())
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), pool, objective, binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "InvalidObjectiveRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "InvalidObjectiveRef")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
}

func TestReconcileUsesDiscoveredObjectiveGVKsForResolution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	xPool := newTestPool()
	xPool.SetGroupVersionKind(inferencePoolGVKs[1])
	xPool.SetName("pool-x")

	xObjective := newTestObjective("objective-shared")
	xObjective.Object["spec"] = map[string]any{
		"poolRef": map[string]any{
			"name":  "pool-x",
			"group": "inference.networking.x-k8s.io",
		},
	}

	k8sObjective := newTestObjective("objective-shared")
	k8sObjective.SetGroupVersionKind(inferenceObjectiveGVKs[1])
	k8sObjective.Object["spec"] = map[string]any{
		"poolRef": map[string]any{
			"name":  "pool-k8s-missing",
			"group": "inference.networking.k8s.io",
		},
	}

	binding := newPerObjectiveBinding("binding-filtered-objective", "objective-shared")
	binding.Spec.PoolRef = kleymv1alpha1.InferencePoolTargetRef{Name: "pool-x"}

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(
				xPool,
				xObjective,
				k8sObjective,
				binding,
			).
			Build(),
		Scheme:                 scheme,
		availableObjectiveGVKs: []schema.GroupVersionKind{inferenceObjectiveGVKs[1]},
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "InvalidObjectiveRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "InvalidObjectiveRef")
}
