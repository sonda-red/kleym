package controller

import (
	"context"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestClusterSPIFFEIDWatchMapsOnlyPersistedManagedOutputNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owned := newPoolOnlyBinding("binding-watch-owned", "")
	setConfirmedClusterSPIFFEID(owned, "recorded-output", "recorded-output-uid")
	pending := newPoolOnlyBinding("binding-watch-pending", "")
	pending.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{Name: "pending-output", ClaimID: "pending-claim"}
	labelOnly := newPoolOnlyBinding("binding-watch-label-only", "")
	setConfirmedClusterSPIFFEID(labelOnly, "different-output", "different-output-uid")
	reconciler := newIndexedWatchTestReconciler(t, owned, pending, labelOnly)

	ownedEvent := managedEventObject("recorded-output")
	ownedEvent.SetLabels(spirecm.ManagedClusterSPIFFEIDLabels(labelOnly))
	assertRequestNames(t, reconciler.mapClusterSPIFFEIDToBindings(ctx, ownedEvent), []string{
		"default/binding-watch-owned",
	})

	pendingEvent := managedEventObject("pending-output")
	assertRequestNames(t, reconciler.mapClusterSPIFFEIDToBindings(ctx, pendingEvent), []string{
		"default/binding-watch-pending",
	})

	labelOnlyEvent := managedEventObject("unrecorded-output")
	labelOnlyEvent.SetLabels(spirecm.ManagedClusterSPIFFEIDLabels(owned))
	assertRequestNames(t, reconciler.mapClusterSPIFFEIDToBindings(ctx, labelOnlyEvent), nil)
}

func TestReverseBindingLookupsUseConfiguredFieldIndexes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	matchingPool := newPoolOnlyBinding("binding-pool-match", "")
	unrelatedPool := newPoolOnlyBinding("binding-pool-other", "")
	unrelatedPool.Spec.PoolRef.Name = "pool-other"
	matchingOutput := newPoolOnlyBinding("binding-output-match", "")
	setConfirmedClusterSPIFFEID(matchingOutput, "recorded-output", "recorded-output-uid")
	unrelatedOutput := newPoolOnlyBinding("binding-output-other", "")
	unrelatedOutput.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{
		Name:    "other-output",
		ClaimID: "other-output-claim",
	}
	reconciler := newIndexedWatchTestReconciler(t, matchingPool, unrelatedPool, matchingOutput, unrelatedOutput)

	poolBindings, err := reconciler.listBindingsReferencingPool(ctx, testNamespace, "pool-a")
	if err != nil {
		t.Fatalf("list bindings by pool reference: %v", err)
	}
	assertBindingNames(t, poolBindings, []string{"binding-output-match", "binding-output-other", "binding-pool-match"})

	outputBindings, err := reconciler.listBindingsByManagedClusterSPIFFEIDName(ctx, "recorded-output")
	if err != nil {
		t.Fatalf("list bindings by managed output name: %v", err)
	}
	assertBindingNames(t, outputBindings, []string{"binding-output-match"})
}

func TestReverseBindingLookupsReturnMissingIndexErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-missing-index", "")
	setConfirmedClusterSPIFFEID(binding, "recorded-output", "recorded-output-uid")
	scheme := newControllerTestScheme(t)
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding).Build(),
	}

	if _, err := reconciler.listBindingsReferencingPool(ctx, testNamespace, "pool-a"); err == nil {
		t.Fatal("list bindings by pool reference error = nil, want missing-index error")
	}
	if _, err := reconciler.listBindingsByManagedClusterSPIFFEIDName(ctx, "recorded-output"); err == nil {
		t.Fatal("list bindings by managed output name error = nil, want missing-index error")
	}
}

func TestClusterSPIFFEIDWatchDeletionRequeuesRecordedBindingAndRestoresReady(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-watch-delete", "")
	reconciler := newIndexedWatchTestReconciler(t, newTestPool(), binding)
	reconcileBinding(t, ctx, reconciler, binding.Name)

	ready := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertSuccessConditionSet(t, ready)
	recordedName := confirmedClusterSPIFFEIDName(ready)
	if recordedName == "" {
		t.Fatal("ownedClusterSPIFFEID name was not recorded")
	}

	deleted := &unstructured.Unstructured{}
	deleted.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: recordedName}, deleted); err != nil {
		t.Fatalf("get managed output before direct delete: %v", err)
	}
	if err := reconciler.Delete(ctx, deleted); err != nil {
		t.Fatalf("directly delete managed output: %v", err)
	}

	requests := reconciler.mapClusterSPIFFEIDToBindings(ctx, deleted)
	assertRequestNames(t, requests, []string{"default/binding-watch-delete"})
	if _, err := reconciler.Reconcile(ctx, requests[0]); err != nil {
		t.Fatalf("reconcile from managed-output delete watch returned error: %v", err)
	}

	recovered := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertSuccessConditionSet(t, recovered)
	if confirmedClusterSPIFFEIDName(recovered) != recordedName {
		t.Fatalf("ownedClusterSPIFFEID.name = %q, want %q", confirmedClusterSPIFFEIDName(recovered), recordedName)
	}
	if recovered.Status.RenderedClusterSPIFFEID == nil {
		t.Fatal("renderedClusterSPIFFEID was not restored")
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: recordedName}, current); err != nil {
		t.Fatalf("managed output was not recreated after watch-triggered reconcile: %v", err)
	}
}

func TestClusterSPIFFEIDWatchSpecDriftRequeuesRecordedBindingAndRestoresReady(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-watch-drift", "")
	reconciler := newIndexedWatchTestReconciler(t, newTestPool(), binding)
	reconcileBinding(t, ctx, reconciler, binding.Name)

	ready := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertSuccessConditionSet(t, ready)
	recordedName := confirmedClusterSPIFFEIDName(ready)
	if recordedName == "" {
		t.Fatal("ownedClusterSPIFFEID name was not recorded")
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: recordedName}, current); err != nil {
		t.Fatalf("get managed output before drift: %v", err)
	}

	oldEvent := current.DeepCopy()
	oldEvent.SetGeneration(1)
	newEvent := current.DeepCopy()
	newEvent.SetGeneration(2)
	if !reconcileWatchPredicate().Update(event.UpdateEvent{ObjectOld: oldEvent, ObjectNew: newEvent}) {
		t.Fatal("ClusterSPIFFEID spec-generation drift would not pass reconcile watch predicate")
	}

	drifted := current.DeepCopy()
	drifted.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          "spiffe://drifted.example/workload",
		"podSelector":               map[string]any{"matchLabels": map[string]any{"app": "drifted"}},
		"workloadSelectorTemplates": []any{"k8s:ns:default", "k8s:sa:drifted"},
	}
	if err := reconciler.Update(ctx, drifted); err != nil {
		t.Fatalf("drift managed output: %v", err)
	}

	requests := reconciler.mapClusterSPIFFEIDToBindings(ctx, drifted)
	assertRequestNames(t, requests, []string{"default/binding-watch-drift"})
	if _, err := reconciler.Reconcile(ctx, requests[0]); err != nil {
		t.Fatalf("reconcile from managed-output drift watch returned error: %v", err)
	}

	recovered := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertSuccessConditionSet(t, recovered)
	identity, err := reconciler.renderIdentityForBinding(ctx, recovered)
	if err != nil {
		t.Fatalf("render desired identity after drift recovery: %v", err)
	}
	desired := spirecm.DesiredClusterSPIFFEID(recovered, identity, "")

	converged := &unstructured.Unstructured{}
	converged.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: recordedName}, converged); err != nil {
		t.Fatalf("get managed output after drift recovery: %v", err)
	}
	if !spirecm.ClusterSPIFFEIDInSync(converged, desired) {
		currentSpec, _, _ := unstructured.NestedMap(converged.Object, "spec")
		desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
		t.Fatalf("managed output not restored to desired spec: current=%#v desired=%#v", currentSpec, desiredSpec)
	}
}

func newIndexedWatchTestReconciler(t *testing.T, objects ...client.Object) *InferenceIdentityBindingReconciler {
	t.Helper()

	scheme := newControllerTestScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexPoolRefName, bindingPoolRefNameIndexValue).
		WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexManagedClusterSPIFFEIDName, bindingClusterSPIFFEIDNameIndexValues).
		WithObjects(objects...).
		Build()
	return &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: withFakeClusterSPIFFEIDUIDs(fakeClient),
	}
}

func managedEventObject(name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	object.SetName(name)
	return object
}

func assertRequestNames(t *testing.T, requests []reconcile.Request, want []string) {
	t.Helper()

	got := make([]string, 0, len(requests))
	for _, request := range requests {
		got = append(got, request.String())
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("requests = %v, want %v", got, want)
		}
	}
}

func assertBindingNames(t *testing.T, bindings []*kleymv1alpha1.InferenceIdentityBinding, want []string) {
	t.Helper()

	got := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		got = append(got, binding.Name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("bindings = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("bindings = %v, want %v", got, want)
		}
	}
}
