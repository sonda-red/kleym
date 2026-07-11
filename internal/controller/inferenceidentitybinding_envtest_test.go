package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestObservedGenerationAdvancesAfterBindingSpecChange(t *testing.T) {
	ctx := context.Background()
	nameSuffix := time.Now().Format("20060102150405.000000000")
	poolName := fmt.Sprintf("pool-observed-%s", nameSuffix)
	bindingName := fmt.Sprintf("binding-observed-%s", nameSuffix)

	pool := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{"app": "model-server"},
			},
		},
	}}
	pool.SetGroupVersionKind(inferencePoolGVKs[0])
	pool.SetNamespace(testNamespace)
	pool.SetName(poolName)
	if err := k8sClient.Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(func() {
		if err := k8sClient.Delete(ctx, pool); err != nil && !errors.IsNotFound(err) {
			t.Errorf("delete pool: %v", err)
		}
	})

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: bindingName},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: poolName},
			ServiceAccountName: "inference-sa",
			IdentityBoundary:   testIdentityBoundary,
		},
	}
	if err := k8sClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	key := types.NamespacedName{Namespace: testNamespace, Name: bindingName}
	t.Cleanup(func() {
		current := &kleymv1alpha1.InferenceIdentityBinding{}
		if err := k8sClient.Get(ctx, key, current); err != nil {
			if !errors.IsNotFound(err) {
				t.Errorf("get binding for cleanup: %v", err)
			}
			return
		}
		if err := k8sClient.Delete(ctx, current); err != nil {
			t.Errorf("delete binding: %v", err)
			return
		}
		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient, Scheme: k8sClient.Scheme()}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
				t.Errorf("reconcile binding cleanup: %v", err)
				return
			}
			if err := k8sClient.Get(ctx, key, current); errors.IsNotFound(err) {
				return
			} else if err != nil {
				t.Errorf("get binding after cleanup reconcile: %v", err)
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("binding still exists after cleanup")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient, Scheme: k8sClient.Scheme()}
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	fetched := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, fetched); err != nil {
		t.Fatalf("get reconciled binding: %v", err)
	}
	previousGeneration := fetched.Generation

	fetched.Spec.ServiceAccountName = "inference-sa-v2"
	if err := k8sClient.Update(ctx, fetched); err != nil {
		t.Fatalf("update binding spec: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("reconcile updated binding: %v", err)
	}

	if err := k8sClient.Get(ctx, key, fetched); err != nil {
		t.Fatalf("get updated binding: %v", err)
	}
	if fetched.Generation <= previousGeneration {
		t.Fatalf("generation = %d, want greater than %d", fetched.Generation, previousGeneration)
	}

	for _, conditionType := range []string{
		conditionTypeReady,
		conditionTypeInvalidRef,
		conditionTypeUnsafeSelector,
		conditionTypeConflict,
		conditionTypeRenderFailure,
	} {
		condition := meta.FindStatusCondition(fetched.Status.Conditions, conditionType)
		if condition == nil {
			t.Errorf("missing condition %s", conditionType)
			continue
		}
		if condition.ObservedGeneration != fetched.Generation {
			t.Errorf("condition %s observedGeneration = %d, want %d", conditionType, condition.ObservedGeneration, fetched.Generation)
		}
	}
}
