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
	reconciler := &InferenceIdentityBindingReconciler{
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
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objective, binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "InvalidPoolRef")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "InvalidPoolRef")
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

	reconciler := &InferenceIdentityBindingReconciler{
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

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "TargetPoolNotFound")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "TargetPoolNotFound")
}
