package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestReconcileConflictWithdrawsOutputsAndPeersConverge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	reconciler := newConflictTestReconciler(t, newTestPool(), testPoolNamed("pool-b"), newPoolOnlyBinding("binding-a", ""))

	reconcileBinding(t, ctx, reconciler, "binding-a")
	peer := newPoolOnlyBinding("binding-b", "")
	peer.Spec.PoolRef.Name = "pool-b"
	if err := reconciler.Create(ctx, peer); err != nil {
		t.Fatalf("create peer binding: %v", err)
	}

	reconcileBinding(t, ctx, reconciler, peer.Name)
	reconcileBinding(t, ctx, reconciler, "binding-a")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
	assertBindingConflict(t, ctx, reconciler.Client, "binding-a", "binding-b", identity.CauseBoundaryValueReuse)
	assertBindingConflict(t, ctx, reconciler.Client, "binding-b", "binding-a", identity.CauseBoundaryValueReuse)

	currentPeer := fetchBinding(t, ctx, reconciler.Client, peer.Name)
	currentPeer.Spec.IdentityBoundary.LabelValue = "decode"
	if err := reconciler.Update(ctx, currentPeer); err != nil {
		t.Fatalf("make peer exclusive: %v", err)
	}
	reconcileBinding(t, ctx, reconciler, "binding-a")
	reconcileBinding(t, ctx, reconciler, peer.Name)
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 2)
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved)
	assertConditionStatus(t, ctx, reconciler.Client, peer.Name, conditionTypeReady, metav1.ConditionTrue, conditionReasonReconciled)
}

func TestReconcileRestartConvergesDuplicateIdentityClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := newPoolOnlyBinding("binding-duplicate-a", "")
	second := newPoolOnlyBinding("binding-duplicate-b", "")
	reconciler := newConflictTestReconciler(t, newTestPool(), first, second)

	reconcileBinding(t, ctx, reconciler, first.Name)
	current := fetchBinding(t, ctx, reconciler.Client, first.Name)
	assertPrimaryFailureCondition(t, current, conditionTypeConflict, conditionReasonDuplicateIdentityBinding)
	if len(current.Status.Conflicts) != 1 || current.Status.Conflicts[0].Cause != string(identity.CauseDuplicateSPIFFEID) {
		t.Fatalf("conflicts = %#v, want DuplicateSPIFFEID", current.Status.Conflicts)
	}
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
}

func TestDeletingConflictPeerBlocksRecoveryUntilOutputAbsence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := newPoolOnlyBinding("binding-delete-block-a", "")
	second := newPoolOnlyBinding("binding-delete-block-b", "")
	second.Spec.PoolRef.Name = "pool-b"
	second.Spec.IdentityBoundary.LabelValue = "decode"
	reconciler := newConflictTestReconciler(t, newTestPool(), testPoolNamed("pool-b"), first, second)
	reconcileBinding(t, ctx, reconciler, first.Name)
	reconcileBinding(t, ctx, reconciler, second.Name)

	currentSecond := fetchBinding(t, ctx, reconciler.Client, second.Name)
	currentSecond.Spec.IdentityBoundary.LabelValue = "prefill"
	if err := reconciler.Update(ctx, currentSecond); err != nil {
		t.Fatalf("make peer conflicting: %v", err)
	}
	managed := managedOutputForBinding(t, ctx, reconciler.Client, currentSecond)
	managed.SetFinalizers([]string{"test.finalizer/hold"})
	if err := reconciler.Update(ctx, managed); err != nil {
		t.Fatalf("hold peer output deletion: %v", err)
	}
	if err := reconciler.Delete(ctx, currentSecond); err != nil {
		t.Fatalf("delete conflicting peer: %v", err)
	}

	result := reconcileBinding(t, ctx, reconciler, first.Name)
	if result.RequeueAfter != deleteVerificationRequeueAfter {
		t.Fatalf("requeueAfter = %s, want %s", result.RequeueAfter, deleteVerificationRequeueAfter)
	}
	currentFirst := fetchBinding(t, ctx, reconciler.Client, first.Name)
	conflict := meta.FindStatusCondition(currentFirst.Status.Conditions, conditionTypeConflict)
	if conflict == nil || conflict.Status != metav1.ConditionUnknown || conflict.Reason != conditionReasonInitializing {
		t.Fatalf("conflict before peer output absence = %#v, want Unknown/Initializing", conflict)
	}
	if len(currentFirst.Status.ComputedSpiffeIDs) != 0 || currentFirst.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("pending conflict retained rendered output status: %#v", currentFirst.Status)
	}

	managed = managedOutputForBinding(t, ctx, reconciler.Client, currentSecond)
	managed.SetFinalizers(nil)
	if err := reconciler.Update(ctx, managed); err != nil {
		t.Fatalf("release peer output deletion: %v", err)
	}
	reconcileBinding(t, ctx, reconciler, first.Name)
	assertConditionStatus(t, ctx, reconciler.Client, first.Name, conditionTypeReady, metav1.ConditionTrue, conditionReasonReconciled)
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func TestConflictOutputDeletionFailureReturnsAndDoesNotSettleConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := newPoolOnlyBinding("binding-delete-error-a", "")
	base := newConflictTestReconciler(t, newTestPool(), testPoolNamed("pool-b"), first)
	reconcileBinding(t, ctx, base, first.Name)
	second := newPoolOnlyBinding("binding-delete-error-b", "")
	second.Spec.PoolRef.Name = "pool-b"
	if err := base.Create(ctx, second); err != nil {
		t.Fatalf("create conflicting peer: %v", err)
	}

	deleteErr := errors.New("delete managed output failed")
	baseWithWatch, ok := base.Client.(client.WithWatch)
	if !ok {
		t.Fatalf("fake client does not implement client.WithWatch")
	}
	wrapped := interceptor.NewClient(baseWithWatch, interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
			if object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				return deleteErr
			}
			return cli.Delete(ctx, object, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped, Scheme: base.Scheme}
	_, err := reconciler.Reconcile(ctx, bindingRequest(second.Name))
	if !errors.Is(err, deleteErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, deleteErr)
	}
	current := fetchBinding(t, ctx, wrapped, second.Name)
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	if conflict := meta.FindStatusCondition(current.Status.Conditions, conditionTypeConflict); conflict != nil && conflict.Status == metav1.ConditionTrue {
		t.Fatalf("Conflict=True reported after deletion failure: %#v", conflict)
	}
	assertClusterSPIFFEIDCount(t, ctx, wrapped, 1)
}

func TestConflictCleanupUsesOwnershipRetainedAfterUpdateFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	first := newPoolOnlyBinding("binding-failure-conflict-a", "")
	base := newConflictTestReconciler(t, newTestPool(), testPoolNamed("pool-b"), first)
	reconcileBinding(t, ctx, base, first.Name)
	current := fetchBinding(t, ctx, base.Client, first.Name)
	recordedName := current.Status.OwnedClusterSPIFFEIDName
	managed := &unstructured.Unstructured{}
	managed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := base.Get(ctx, types.NamespacedName{Name: recordedName}, managed); err != nil {
		t.Fatalf("get managed output: %v", err)
	}
	managed.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://drifted.example/workload"}
	if err := base.Update(ctx, managed); err != nil {
		t.Fatalf("drift managed output: %v", err)
	}

	updateErr := errors.New("update managed output failed")
	failUpdate := true
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Update: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.UpdateOption) error {
			if failUpdate && object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				failUpdate = false
				return updateErr
			}
			return cli.Update(ctx, object, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped, Scheme: base.Scheme}
	if _, err := reconciler.Reconcile(ctx, bindingRequest(first.Name)); !errors.Is(err, updateErr) {
		t.Fatalf("failure Reconcile error = %v, want %v", err, updateErr)
	}
	failed := fetchBinding(t, ctx, wrapped, first.Name)
	if failed.Status.OwnedClusterSPIFFEIDName != recordedName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after failure, want %q", failed.Status.OwnedClusterSPIFFEIDName, recordedName)
	}

	second := newPoolOnlyBinding("binding-failure-conflict-b", "")
	second.Spec.PoolRef.Name = "pool-b"
	if err := wrapped.Create(ctx, second); err != nil {
		t.Fatalf("create conflicting binding: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, bindingRequest(first.Name)); err != nil {
		t.Fatalf("conflict Reconcile returned error: %v", err)
	}
	conflicted := fetchBinding(t, ctx, wrapped, first.Name)
	assertPrimaryFailureCondition(t, conflicted, conditionTypeConflict, conditionReasonIdentityBoundaryConflict)
	if conflicted.Status.OwnedClusterSPIFFEIDName != "" {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after confirmed conflict cleanup, want empty", conflicted.Status.OwnedClusterSPIFFEIDName)
	}
	assertClusterSPIFFEIDCount(t, ctx, wrapped, 0)
}

func TestReconcileRefusesForeignDeterministicOutputWithManagedLabels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-unrelated-output", "")
	pool := newTestPool()
	preflight := newConflictTestReconciler(t, pool.DeepCopy(), binding.DeepCopy())
	plan, err := preflight.renderIdentity(binding, pool)
	if err != nil {
		t.Fatalf("render desired identity: %v", err)
	}
	foreign := spirecm.DesiredClusterSPIFFEID(binding, plan, "")
	foreign.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/workload"}
	wantLabels := foreign.GetLabels()
	wantSpec, _, _ := unstructured.NestedMap(foreign.Object, "spec")

	reconciler := newConflictTestReconciler(t, pool, binding, foreign)
	_, err = reconciler.Reconcile(ctx, bindingRequest(binding.Name))
	if err == nil {
		t.Fatal("Reconcile error = nil, want foreign output refusal")
	}
	current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	observed := &unstructured.Unstructured{}
	observed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if getErr := reconciler.Get(ctx, types.NamespacedName{Name: foreign.GetName()}, observed); getErr != nil {
		t.Fatalf("get foreign output: %v", getErr)
	}
	observedSpec, _, _ := unstructured.NestedMap(observed.Object, "spec")
	if !reflect.DeepEqual(observed.GetLabels(), wantLabels) || !reflect.DeepEqual(observedSpec, wantSpec) {
		t.Fatalf("foreign output was modified: labels=%v spec=%v", observed.GetLabels(), observedSpec)
	}
}

func TestPeerMappingCoversBindingAndReferencedPoolChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := newPoolOnlyBinding("binding-map-a", "")
	second := newPoolOnlyBinding("binding-map-b", "")
	second.Spec.PoolRef.Name = "pool-b"
	reconciler := newConflictTestReconciler(t, first, second)

	poolRequests := reconciler.mapPoolToBindings(ctx, newTestPool())
	if len(poolRequests) != 2 {
		t.Fatalf("pool change requests = %v, want both namespace peers", poolRequests)
	}
	bindingRequests := reconciler.mapBindingToPeers(ctx, first)
	if len(bindingRequests) != 2 {
		t.Fatalf("binding change requests = %v, want both namespace peers", bindingRequests)
	}
}

func TestPoolMutationRequeuesPeerAndConvergesConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := newPoolOnlyBinding("binding-pool-mutation-a", "")
	second := newPoolOnlyBinding("binding-pool-mutation-b", "")
	second.Spec.PoolRef.Name = "pool-b"
	second.Spec.IdentityBoundary.LabelValue = "decode"
	poolB := testPoolNamed("pool-b")
	reconciler := newConflictTestReconciler(t, newTestPool(), poolB, first, second)
	reconcileBinding(t, ctx, reconciler, first.Name)
	reconcileBinding(t, ctx, reconciler, second.Name)
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 2)

	currentPool := &unstructured.Unstructured{}
	currentPool.SetGroupVersionKind(inferencePoolGVKs[0])
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: poolB.GetName()}, currentPool); err != nil {
		t.Fatalf("get pool for mutation: %v", err)
	}
	currentPool.Object["spec"] = map[string]any{
		"selector": map[string]any{
			"matchLabels":      map[string]any{"app": "model-server"},
			"matchExpressions": []any{},
		},
	}
	if err := reconciler.Update(ctx, currentPool); err != nil {
		t.Fatalf("update pool selector: %v", err)
	}
	if requests := reconciler.mapPoolToBindings(ctx, currentPool); len(requests) != 2 {
		t.Fatalf("pool mutation requests = %v, want both conflict peers", requests)
	}

	result := reconcileBinding(t, ctx, reconciler, first.Name)
	if result.RequeueAfter != deleteVerificationRequeueAfter {
		t.Fatalf("peer recovery requeueAfter = %s, want output-absence verification", result.RequeueAfter)
	}
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
	reconcileBinding(t, ctx, reconciler, second.Name)
	reconcileBinding(t, ctx, reconciler, first.Name)
	assertConditionStatus(t, ctx, reconciler.Client, second.Name, conditionTypeUnsafeSelector, metav1.ConditionTrue, identity.ReasonInvalidPoolSelector)
	assertConditionStatus(t, ctx, reconciler.Client, first.Name, conditionTypeReady, metav1.ConditionTrue, conditionReasonReconciled)
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func newConflictTestReconciler(t *testing.T, objects ...client.Object) *InferenceIdentityBindingReconciler {
	t.Helper()
	scheme := newControllerTestScheme(t)
	return &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexPoolRefName, bindingPoolRefNameIndexValue).
			WithIndex(&kleymv1alpha1.InferenceIdentityBinding{}, fieldIndexManagedClusterSPIFFEIDName, bindingClusterSPIFFEIDNameIndexValues).
			WithObjects(objects...).
			Build(),
		Scheme: scheme,
	}
}

func testPoolNamed(name string) *unstructured.Unstructured {
	pool := newTestPool()
	pool.SetName(name)
	return pool
}

func reconcileBinding(
	t *testing.T,
	ctx context.Context,
	reconciler *InferenceIdentityBindingReconciler,
	name string,
) reconcile.Result {
	t.Helper()
	result, err := reconciler.Reconcile(ctx, bindingRequest(name))
	if err != nil {
		t.Fatalf("Reconcile %s returned error: %v", name, err)
	}
	return result
}

func bindingRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: name}}
}

func assertBindingConflict(
	t *testing.T,
	ctx context.Context,
	cli client.Client,
	name string,
	peerName string,
	cause identity.ConflictCause,
) {
	t.Helper()
	binding := fetchBinding(t, ctx, cli, name)
	assertPrimaryFailureCondition(t, binding, conditionTypeConflict, conditionReasonIdentityBoundaryConflict)
	if binding.Status.IdentityBoundary == nil || binding.Status.IdentityBoundary.LabelValue != "prefill" {
		t.Fatalf("identityBoundary = %#v, want retained prefill boundary", binding.Status.IdentityBoundary)
	}
	if len(binding.Status.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v, want one peer", binding.Status.Conflicts)
	}
	conflict := binding.Status.Conflicts[0]
	if conflict.BindingRef.Name != peerName || conflict.Cause != string(cause) || conflict.SpiffeID == "" {
		t.Fatalf("conflict = %#v, want peer=%s cause=%s", conflict, peerName, cause)
	}
	if len(binding.Status.ComputedSpiffeIDs) != 0 || len(binding.Status.RenderedSelectors) != 0 || binding.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("rendered output status retained during conflict: %#v", binding.Status)
	}
}

func managedOutputForBinding(
	t *testing.T,
	ctx context.Context,
	cli client.Client,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) *unstructured.Unstructured {
	t.Helper()
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))
	if err := cli.List(ctx, list, client.MatchingLabels(spirecm.ManagedClusterSPIFFEIDLabels(binding))); err != nil {
		t.Fatalf("list managed output for %s: %v", binding.Name, err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("managed output count for %s = %d, want 1", binding.Name, len(list.Items))
	}
	return list.Items[0].DeepCopy()
}
