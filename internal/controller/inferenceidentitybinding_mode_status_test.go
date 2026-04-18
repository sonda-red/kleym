package controller

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestModeFlipKeepsComputedSpiffeIDsInSyncWithManagedClusterSPIFFEIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPerObjectiveBinding("binding-mode-flip", "objective-a")
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

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("initial Reconcile returned error: %v", err)
	}

	currentBinding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, request.NamespacedName, currentBinding); err != nil {
		t.Fatalf("failed to fetch binding after initial reconcile: %v", err)
	}

	if len(currentBinding.Status.ComputedSpiffeIDs) != 1 {
		t.Fatalf("computedSpiffeIDs count after initial reconcile = %d, want 1", len(currentBinding.Status.ComputedSpiffeIDs))
	}
	if currentBinding.Status.ComputedSpiffeIDs[0].Mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		t.Fatalf(
			"computedSpiffeIDs[0].Mode after initial reconcile = %q, want %q",
			currentBinding.Status.ComputedSpiffeIDs[0].Mode,
			kleymv1alpha1.InferenceIdentityBindingModePerObjective,
		)
	}

	perObjectiveIDs, perObjectiveNames, err := managedSPIFFEIDsForBinding(ctx, reconciler, currentBinding)
	if err != nil {
		t.Fatalf("failed to list managed ClusterSPIFFEIDs after initial reconcile: %v", err)
	}
	if len(perObjectiveIDs) != 1 || len(perObjectiveNames) != 1 {
		t.Fatalf("managed ClusterSPIFFEIDs after initial reconcile = ids:%d names:%d, want 1", len(perObjectiveIDs), len(perObjectiveNames))
	}
	if perObjectiveIDs[0] != currentBinding.Status.ComputedSpiffeIDs[0].SpiffeID {
		t.Fatalf(
			"computedSpiffeIDs and managed ClusterSPIFFEID mismatch after initial reconcile: status=%q managed=%q",
			currentBinding.Status.ComputedSpiffeIDs[0].SpiffeID,
			perObjectiveIDs[0],
		)
	}
	originalManagedName := perObjectiveNames[0]

	currentBinding.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePoolOnly
	currentBinding.Spec.ContainerDiscriminator = nil
	if err := reconciler.Update(ctx, currentBinding); err != nil {
		t.Fatalf("failed to update binding mode to PoolOnly: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("reconcile after mode flip returned error: %v", err)
	}

	if err := reconciler.Get(ctx, request.NamespacedName, currentBinding); err != nil {
		t.Fatalf("failed to fetch binding after mode flip reconcile: %v", err)
	}
	if len(currentBinding.Status.ComputedSpiffeIDs) != 1 {
		t.Fatalf("computedSpiffeIDs count after mode flip = %d, want 1", len(currentBinding.Status.ComputedSpiffeIDs))
	}
	if currentBinding.Status.ComputedSpiffeIDs[0].Mode != kleymv1alpha1.InferenceIdentityBindingModePoolOnly {
		t.Fatalf(
			"computedSpiffeIDs[0].Mode after mode flip = %q, want %q",
			currentBinding.Status.ComputedSpiffeIDs[0].Mode,
			kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		)
	}

	poolOnlyIDs, poolOnlyNames, err := managedSPIFFEIDsForBinding(ctx, reconciler, currentBinding)
	if err != nil {
		t.Fatalf("failed to list managed ClusterSPIFFEIDs after mode flip: %v", err)
	}
	if len(poolOnlyIDs) != 1 || len(poolOnlyNames) != 1 {
		t.Fatalf("managed ClusterSPIFFEIDs after mode flip = ids:%d names:%d, want 1", len(poolOnlyIDs), len(poolOnlyNames))
	}
	if poolOnlyIDs[0] != currentBinding.Status.ComputedSpiffeIDs[0].SpiffeID {
		t.Fatalf(
			"computedSpiffeIDs and managed ClusterSPIFFEID mismatch after mode flip: status=%q managed=%q",
			currentBinding.Status.ComputedSpiffeIDs[0].SpiffeID,
			poolOnlyIDs[0],
		)
	}
	if poolOnlyNames[0] == originalManagedName {
		t.Fatalf("managed ClusterSPIFFEID name did not change after mode flip: %q", poolOnlyNames[0])
	}
}

func managedSPIFFEIDsForBinding(
	ctx context.Context,
	reconciler *InferenceIdentityBindingReconciler,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]string, []string, error) {
	managedObjects, err := reconciler.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]string, 0, len(managedObjects))
	names := make([]string, 0, len(managedObjects))
	for _, object := range managedObjects {
		spiffeID, found, nestedErr := unstructured.NestedString(object.Object, "spec", "spiffeIDTemplate")
		if nestedErr != nil {
			return nil, nil, nestedErr
		}
		if !found {
			return nil, nil, fmt.Errorf("managed ClusterSPIFFEID %q missing spec.spiffeIDTemplate", object.GetName())
		}

		ids = append(ids, spiffeID)
		names = append(names, object.GetName())
	}

	sort.Strings(ids)
	sort.Strings(names)
	return ids, names, nil
}
