package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// TestPendingClusterSPIFFEIDStatusTransitionsAtomicallyToOwned verifies the CRD
// accepts the controller's durable ownership handoff without an invalid overlap.
func TestPendingClusterSPIFFEIDStatusTransitionsAtomicallyToOwned(t *testing.T) {
	ctx := context.Background()
	name := "binding-ownership-transition-" + time.Now().Format("150405.000000000")
	binding := newPoolOnlyBinding(name, "")
	if err := k8sClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, binding) })

	key := types.NamespacedName{Namespace: testNamespace, Name: name}
	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, current); err != nil {
		t.Fatalf("get binding: %v", err)
	}

	// Persist the pre-create claim as the only ownership record.
	base := current.DeepCopy()
	current.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{
		Name:    "pending-output",
		ClaimID: "pending-claim",
	}
	if err := k8sClient.Status().Patch(ctx, current, client.MergeFrom(base)); err != nil {
		t.Fatalf("persist pending ownership: %v", err)
	}

	pending := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, pending); err != nil {
		t.Fatalf("get pending binding: %v", err)
	}
	if pending.Status.PendingClusterSPIFFEID == nil || pending.Status.OwnedClusterSPIFFEID != nil {
		t.Fatalf("pending status = pending %#v owned %#v, want pending only", pending.Status.PendingClusterSPIFFEID, pending.Status.OwnedClusterSPIFFEID)
	}

	// Replace the claim with confirmed name-and-UID ownership in one patch.
	base = pending.DeepCopy()
	pending.Status.PendingClusterSPIFFEID = nil
	pending.Status.OwnedClusterSPIFFEID = &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{
		Name: "pending-output",
		UID:  "owned-output-uid",
	}
	if err := k8sClient.Status().Patch(ctx, pending, client.MergeFrom(base)); err != nil {
		t.Fatalf("replace pending ownership with confirmed ownership: %v", err)
	}

	owned := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, owned); err != nil {
		t.Fatalf("get owned binding: %v", err)
	}
	if owned.Status.PendingClusterSPIFFEID != nil || owned.Status.OwnedClusterSPIFFEID == nil {
		t.Fatalf("owned status = pending %#v owned %#v, want owned only", owned.Status.PendingClusterSPIFFEID, owned.Status.OwnedClusterSPIFFEID)
	}
}

func TestOwnedClusterSPIFFEIDStatusRejectsEmptyUID(t *testing.T) {
	ctx := context.Background()
	name := "binding-empty-owned-uid-" + time.Now().Format("150405.000000000")
	binding := newPoolOnlyBinding(name, "")
	if err := k8sClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, binding) })

	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), current); err != nil {
		t.Fatalf("get binding: %v", err)
	}
	base := current.DeepCopy()
	current.Status.OwnedClusterSPIFFEID = &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{Name: "owned-output", UID: ""}
	if err := k8sClient.Status().Patch(ctx, current, client.MergeFrom(base)); !errors.IsInvalid(err) {
		t.Fatalf("empty owned UID status patch error = %v, want Invalid", err)
	}
}

func TestOwnershipStatusRejectsPendingAndOwnedTogether(t *testing.T) {
	ctx := context.Background()
	name := "binding-overlapping-ownership-" + time.Now().Format("150405.000000000")
	binding := newPoolOnlyBinding(name, "")
	if err := k8sClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, binding) })

	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), current); err != nil {
		t.Fatalf("get binding: %v", err)
	}
	base := current.DeepCopy()
	current.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{Name: "output", ClaimID: "claim"}
	current.Status.OwnedClusterSPIFFEID = &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{Name: "output", UID: "owned-uid"}
	if err := k8sClient.Status().Patch(ctx, current, client.MergeFrom(base)); !errors.IsInvalid(err) {
		t.Fatalf("overlapping ownership status patch error = %v, want Invalid", err)
	}
}

func TestDeleteAndRecreateSameNameIsNotOwned(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000000")
	poolName := "pool-incarnation-" + suffix
	bindingName := "binding-incarnation-" + suffix

	pool := newTestPool()
	pool.SetName(poolName)
	if err := k8sClient.Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}
	binding := newPoolOnlyBinding(bindingName, "")
	binding.Spec.PoolRef.Name = poolName
	if err := k8sClient.Create(ctx, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	key := types.NamespacedName{Namespace: testNamespace, Name: bindingName}
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient}
	t.Cleanup(func() {
		current := &kleymv1alpha1.InferenceIdentityBinding{}
		if err := k8sClient.Get(ctx, key, current); err == nil {
			_ = k8sClient.Delete(ctx, current)
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		}
		_ = k8sClient.Delete(ctx, pool)
	})

	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	ready := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, ready); err != nil {
		t.Fatalf("get ready binding: %v", err)
	}
	if ready.Status.OwnedClusterSPIFFEID == nil || ready.Status.OwnedClusterSPIFFEID.UID == "" {
		t.Fatalf("confirmed ownership = %#v, want name and UID", ready.Status.OwnedClusterSPIFFEID)
	}

	owned := fetchClusterSPIFFEID(t, ctx, k8sClient, ready.Status.OwnedClusterSPIFFEID.Name)
	oldUID := owned.GetUID()
	if err := k8sClient.Delete(ctx, owned); err != nil {
		t.Fatalf("delete owned ClusterSPIFFEID: %v", err)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: owned.GetName()}, owned); !errors.IsNotFound(err) {
		t.Fatalf("confirm owned ClusterSPIFFEID deletion: %v", err)
	}

	foreign := &unstructured.Unstructured{}
	foreign.SetGroupVersionKind(clusterSPIFFEIDGVK)
	foreign.SetName(ready.Status.OwnedClusterSPIFFEID.Name)
	foreign.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/replacement"}
	if err := k8sClient.Create(ctx, foreign); err != nil {
		t.Fatalf("create same-name foreign replacement: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, foreign) })
	if foreign.GetUID() == "" || foreign.GetUID() == oldUID {
		t.Fatalf("foreign UID = %q, want nonempty and different from %q", foreign.GetUID(), oldUID)
	}
	wantSpec, _, _ := unstructured.NestedMap(foreign.Object, "spec")

	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err == nil {
		t.Fatal("reconcile after delete-and-recreate returned nil, want ownership refusal")
	}
	failed := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := k8sClient.Get(ctx, key, failed); err != nil {
		t.Fatalf("get failed binding: %v", err)
	}
	if failed.Status.OwnedClusterSPIFFEID != nil || failed.Status.PendingClusterSPIFFEID != nil {
		t.Fatalf("stale ownership = pending %#v owned %#v, want cleared", failed.Status.PendingClusterSPIFFEID, failed.Status.OwnedClusterSPIFFEID)
	}
	renderFailure := meta.FindStatusCondition(failed.Status.Conditions, conditionTypeRenderFailure)
	if renderFailure == nil || renderFailure.Reason != conditionReasonManagedOutputApplyFailed || !strings.Contains(renderFailure.Message, "does not match confirmed UID") {
		t.Fatalf("RenderFailure = %#v, want ManagedOutputApplyFailed with UID mismatch", renderFailure)
	}

	observed := fetchClusterSPIFFEID(t, ctx, k8sClient, foreign.GetName())
	gotSpec, _, _ := unstructured.NestedMap(observed.Object, "spec")
	if observed.GetUID() != foreign.GetUID() || !reflect.DeepEqual(gotSpec, wantSpec) {
		t.Fatalf("foreign replacement was mutated: UID=%q spec=%#v", observed.GetUID(), gotSpec)
	}
}

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
		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient}
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

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient}
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
