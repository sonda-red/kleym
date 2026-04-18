package controller

import (
	"context"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestMapObjectiveToBindingsTargetsOnlyMatchingBindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexTargetRefName, bindingTargetRefNameIndexValue).
			WithObjects(
				newPerObjectiveBinding("binding-a", "objective-a"),
				newPerObjectiveBinding("binding-b", "objective-b"),
			).
			Build(),
		Scheme: scheme,
	}

	objective := newObjectiveWithPool("objective-a", "pool-a", "")
	requests := reconciler.mapObjectiveToBindings(ctx, objective)

	expected := []string{types.NamespacedName{Namespace: testNamespace, Name: "binding-a"}.String()}
	if got := requestNames(requests); !equalStringSlices(got, expected) {
		t.Fatalf("mapObjectiveToBindings returned %v, want %v", got, expected)
	}
}

func TestMapPoolToBindingsTargetsOnlyBindingsForReferencingObjectives(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexTargetRefName, bindingTargetRefNameIndexValue).
			WithObjects(
				newObjectiveWithPool("objective-a", "pool-a", ""),
				newObjectiveWithPool("objective-b", "pool-b", ""),
				newPerObjectiveBinding("binding-a", "objective-a"),
				newPerObjectiveBinding("binding-b", "objective-b"),
			).
			Build(),
		Scheme: scheme,
	}

	pool := newTestPool()
	pool.SetName("pool-a")
	requests := reconciler.mapPoolToBindings(ctx, pool)

	expected := []string{types.NamespacedName{Namespace: testNamespace, Name: "binding-a"}.String()}
	if got := requestNames(requests); !equalStringSlices(got, expected) {
		t.Fatalf("mapPoolToBindings returned %v, want %v", got, expected)
	}
}

func TestReconcileWatchPredicateSkipsStatusOnlyUpdates(t *testing.T) {
	t.Parallel()

	predicate := reconcileWatchPredicate()
	oldBinding := newPerObjectiveBinding("binding-a", "objective-a")
	oldBinding.Generation = 3

	statusOnly := oldBinding.DeepCopy()
	statusOnly.Status.Conditions = []metav1.Condition{{Type: conditionTypeReady, Status: metav1.ConditionTrue}}
	if predicate.Update(event.UpdateEvent{ObjectOld: oldBinding, ObjectNew: statusOnly}) {
		t.Fatalf("status-only update should not pass predicate")
	}

	specChange := oldBinding.DeepCopy()
	specChange.Generation = 4
	if !predicate.Update(event.UpdateEvent{ObjectOld: oldBinding, ObjectNew: specChange}) {
		t.Fatalf("spec update should pass predicate")
	}

	deleting := oldBinding.DeepCopy()
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	if !predicate.Update(event.UpdateEvent{ObjectOld: oldBinding, ObjectNew: deleting}) {
		t.Fatalf("deletion timestamp transition should pass predicate")
	}
}

func TestReconcileWatchPredicateSkipsControllerStatusPatchEventShape(t *testing.T) {
	t.Parallel()

	predicate := reconcileWatchPredicate()
	oldBinding := newPerObjectiveBinding("binding-a", "objective-a")
	oldBinding.Generation = 7
	oldBinding.ResourceVersion = "10"
	oldBinding.UID = types.UID("11111111-1111-1111-1111-111111111111")

	statusPatched := oldBinding.DeepCopy()
	statusPatched.ResourceVersion = "11"
	statusPatched.ManagedFields = []metav1.ManagedFieldsEntry{
		{
			Manager:     "inferenceidentitybinding-controller",
			Operation:   metav1.ManagedFieldsOperationUpdate,
			APIVersion:  kleymv1alpha1.GroupVersion.String(),
			Subresource: "status",
		},
	}
	statusPatched.Status.ComputedSpiffeIDs = []kleymv1alpha1.ComputedSpiffeIDStatus{
		{
			Mode:     kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			SpiffeID: "spiffe://kleym.sonda.red/ns/default/obj/objective-a",
		},
	}
	statusPatched.Status.Conditions = []metav1.Condition{
		{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: oldBinding.Generation,
			Reason:             "Reconciled",
			Message:            "Binding reconciled",
		},
	}

	if predicate.Update(event.UpdateEvent{
		ObjectOld: oldBinding,
		ObjectNew: statusPatched,
	}) {
		t.Fatalf("controller status patch update should not pass predicate")
	}
}

func newObjectiveWithPool(name, poolName, poolGroup string) *unstructured.Unstructured {
	poolRef := map[string]any{
		"name": poolName,
	}
	if poolGroup != "" {
		poolRef["group"] = poolGroup
	}

	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"poolRef": poolRef,
			},
		},
	}
	objective.SetGroupVersionKind(inferenceObjectiveGVKs[0])
	objective.SetNamespace(testNamespace)
	objective.SetName(name)
	return objective
}

func requestNames(requests []reconcile.Request) []string {
	names := make([]string, 0, len(requests))
	for _, request := range requests {
		names = append(names, request.String())
	}
	sort.Strings(names)
	return names
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
