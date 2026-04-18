package controller

import (
	"context"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestReconcileDeleteWaitsForManagedClusterSPIFFEIDsToDisappear(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPerObjectiveBinding("binding-delete", "objective-a")
	controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
	binding.SetDeletionTimestamp(&metav1.Time{Time: metav1.Now().Time})

	managed := newManagedClusterSPIFFEIDForBinding(binding, "binding-delete-child")
	managed.SetFinalizers([]string{"test.finalizer/hold"})

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(binding, managed).
			Build(),
		Scheme: scheme,
	}

	fetchedBinding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, fetchedBinding); err != nil {
		t.Fatalf("failed to fetch binding: %v", err)
	}

	result, err := reconciler.reconcileDelete(ctx, fetchedBinding)
	if err != nil {
		t.Fatalf("reconcileDelete returned error: %v", err)
	}
	if result.RequeueAfter != deleteVerificationRequeueAfter {
		t.Fatalf("requeueAfter = %s, want %s", result.RequeueAfter, deleteVerificationRequeueAfter)
	}

	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, fetchedBinding); err != nil {
		t.Fatalf("failed to fetch binding after first reconcileDelete: %v", err)
	}
	if !controllerutil.ContainsFinalizer(fetchedBinding, inferenceIdentityBindingFinalizer) {
		t.Fatalf("binding finalizer removed before managed ClusterSPIFFEIDs were gone")
	}

	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, fetchedBinding); err != nil {
		t.Fatalf("failed to refetch binding before idempotency check: %v", err)
	}
	result, err = reconciler.reconcileDelete(ctx, fetchedBinding)
	if err != nil {
		t.Fatalf("idempotent reconcileDelete with unchanged child state returned error: %v", err)
	}
	if result.RequeueAfter != deleteVerificationRequeueAfter {
		t.Fatalf("idempotent requeueAfter = %s, want %s", result.RequeueAfter, deleteVerificationRequeueAfter)
	}

	fetchedManaged := &unstructured.Unstructured{}
	fetchedManaged.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: managed.GetName()}, fetchedManaged); err != nil {
		t.Fatalf("failed to fetch managed ClusterSPIFFEID after first reconcileDelete: %v", err)
	}
	fetchedManaged.SetFinalizers(nil)
	if err := reconciler.Update(ctx, fetchedManaged); err != nil {
		t.Fatalf("failed to clear managed ClusterSPIFFEID finalizer: %v", err)
	}

	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, fetchedBinding); err != nil {
		t.Fatalf("failed to refetch binding before second reconcileDelete: %v", err)
	}
	result, err = reconciler.reconcileDelete(ctx, fetchedBinding)
	if err != nil {
		t.Fatalf("second reconcileDelete returned error: %v", err)
	}
	if result != (reconcile.Result{}) {
		t.Fatalf("second reconcileDelete result = %#v, want empty result", result)
	}

	err = reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, fetchedBinding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("failed to fetch binding after second reconcileDelete: %v", err)
		}
	} else if controllerutil.ContainsFinalizer(fetchedBinding, inferenceIdentityBindingFinalizer) {
		t.Fatalf("binding finalizer should be removed after managed ClusterSPIFFEIDs are gone")
		result, err = reconciler.reconcileDelete(ctx, fetchedBinding)
		if err != nil {
			t.Fatalf("idempotent reconcileDelete returned error: %v", err)
		}
		if result != (reconcile.Result{}) {
			t.Fatalf("idempotent reconcileDelete result = %#v, want empty result", result)
		}
	}
}

func TestReconcileCorrectsClusterSPIFFEIDDriftOnResync(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPerObjectiveBinding("binding-drift", "objective-a")

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(
				newTestPool(),
				newTestObjective("objective-a"),
				binding,
			).
			Build(),
		Scheme: scheme,
	}

	request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name}}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("initial Reconcile returned error: %v", err)
	}

	currentBinding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, request.NamespacedName, currentBinding); err != nil {
		t.Fatalf("failed to fetch binding: %v", err)
	}

	identity, err := reconciler.renderIdentityForBinding(ctx, currentBinding)
	if err != nil {
		t.Fatalf("failed to render desired identity: %v", err)
	}
	desired := desiredClusterSPIFFEID(currentBinding, identity)

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: desired.GetName()}, current); err != nil {
		t.Fatalf("failed to fetch managed ClusterSPIFFEID: %v", err)
	}

	drifted := current.DeepCopy()
	labels := drifted.GetLabels()
	labels["drifted"] = "true"
	drifted.SetLabels(labels)
	drifted.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          "spiffe://drifted.example/ns/default/obj/objective-a",
		"podSelector":               map[string]any{"matchLabels": map[string]any{"app": "drifted"}},
		"workloadSelectorTemplates": []any{"k8s:ns:default", "k8s:sa:drifted"},
	}
	if err := reconciler.Update(ctx, drifted); err != nil {
		t.Fatalf("failed to update drifted ClusterSPIFFEID: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("resync Reconcile returned error: %v", err)
	}

	current = &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: desired.GetName()}, current); err != nil {
		t.Fatalf("failed to fetch corrected ClusterSPIFFEID: %v", err)
	}

	if !clusterSPIFFEIDInSync(current, desired) {
		currentSpec, _, _ := unstructured.NestedMap(current.Object, "spec")
		desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
		t.Fatalf(
			"managed ClusterSPIFFEID was not converged back to desired state: currentSpec=%#v desiredSpec=%#v currentLabels=%#v desiredLabels=%#v",
			currentSpec,
			desiredSpec,
			current.GetLabels(),
			desired.GetLabels(),
		)
	}

	currentSpec, _, _ := unstructured.NestedMap(current.Object, "spec")
	desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	if !reflect.DeepEqual(currentSpec, desiredSpec) {
		t.Fatalf("corrected ClusterSPIFFEID spec mismatch: got %#v want %#v", currentSpec, desiredSpec)
	}
}

func newManagedClusterSPIFFEIDForBinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	name string,
) *unstructured.Unstructured {
	managed := &unstructured.Unstructured{}
	managed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	managed.SetName(name)
	managed.SetLabels(map[string]string{
		managedByLabelKey:        managedByLabelValue,
		bindingNameLabelKey:      binding.Name,
		bindingNamespaceLabelKey: binding.Namespace,
	})
	managed.Object["spec"] = map[string]any{
		"spiffeIDTemplate": "spiffe://example.test/ns/default/obj/example",
	}
	return managed
}
