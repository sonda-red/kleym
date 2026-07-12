package controller

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestManagedOutputOwnershipClassification(t *testing.T) {
	t.Parallel()

	matchingClaim := ownershipFixture("output", "live-uid", "claim-a")
	differentClaim := ownershipFixture("output", "live-uid", "claim-b")
	matchingUID := ownershipFixture("output", "owned-uid", "")
	differentUID := ownershipFixture("output", "foreign-uid", "")
	differentNameClaim := ownershipFixture("different-output", "live-uid", "claim-a")
	differentNameUID := ownershipFixture("different-output", "owned-uid", "")

	cases := map[string]struct {
		record managedOutputRecord
		object *unstructured.Unstructured
		want   managedOutputObservationKind
	}{
		"unclaimed absent":     {record: managedOutputRecord{kind: managedOutputUnclaimed}, want: unclaimedOutputAbsent},
		"unclaimed foreign":    {record: managedOutputRecord{kind: managedOutputUnclaimed}, object: differentUID, want: unclaimedOutputForeign},
		"pending absent":       {record: managedOutputRecord{kind: managedOutputPending, name: "output", claimID: "claim-a"}, want: pendingOutputAbsent},
		"pending matched":      {record: managedOutputRecord{kind: managedOutputPending, name: "output", claimID: "claim-a"}, object: matchingClaim, want: pendingOutputMatched},
		"pending foreign":      {record: managedOutputRecord{kind: managedOutputPending, name: "output", claimID: "claim-a"}, object: differentClaim, want: pendingOutputForeign},
		"pending wrong name":   {record: managedOutputRecord{kind: managedOutputPending, name: "output", claimID: "claim-a"}, object: differentNameClaim, want: pendingOutputForeign},
		"confirmed absent":     {record: managedOutputRecord{kind: managedOutputConfirmed, name: "output", uid: "owned-uid"}, want: confirmedOutputAbsent},
		"confirmed matched":    {record: managedOutputRecord{kind: managedOutputConfirmed, name: "output", uid: "owned-uid"}, object: matchingUID, want: confirmedOutputMatched},
		"confirmed foreign":    {record: managedOutputRecord{kind: managedOutputConfirmed, name: "output", uid: "owned-uid"}, object: differentUID, want: confirmedOutputForeign},
		"confirmed wrong name": {record: managedOutputRecord{kind: managedOutputConfirmed, name: "output", uid: "owned-uid"}, object: differentNameUID, want: confirmedOutputForeign},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := classifyManagedClusterSPIFFEID(tc.record, tc.object).kind; got != tc.want {
				t.Fatalf("classification = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLegacyNameOnlyStatusDoesNotAuthorizeOutput(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	legacyStatus := []byte(`{"status":{"pendingClusterSPIFFEIDName":"legacy-output","ownedClusterSPIFFEIDName":"legacy-output"}}`)
	if err := json.Unmarshal(legacyStatus, binding); err != nil {
		t.Fatalf("decode legacy binding status: %v", err)
	}

	record, err := managedOutputRecordFromStatus(binding)
	if err != nil {
		t.Fatalf("classify legacy binding status: %v", err)
	}
	if record.kind != managedOutputUnclaimed {
		t.Fatalf("legacy ownership record kind = %v, want unclaimed", record.kind)
	}

	live := ownershipFixture("legacy-output", "live-uid", "")
	if got := classifyManagedClusterSPIFFEID(record, live).kind; got != unclaimedOutputForeign {
		t.Fatalf("legacy same-name output classification = %v, want foreign", got)
	}
}

func TestPendingMatchingClaimPromotesWithoutSecondCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-pending-match", "")
	_, output := desiredOwnershipFixture(t, ctx, binding)
	claimID := "claim-pending-match"
	binding.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{Name: output.GetName(), ClaimID: claimID}
	output.SetUID("pending-match-uid")
	spirecm.SetClusterSPIFFEIDOwnershipClaim(output, claimID)

	base := newConflictTestReconciler(t, newTestPool(), binding, output)
	createCalls := 0
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			if object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				createCalls++
			}
			return cli.Create(ctx, object, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	reconcileBinding(t, ctx, reconciler, binding.Name)
	current := fetchBinding(t, ctx, wrapped, binding.Name)
	if current.Status.PendingClusterSPIFFEID != nil || current.Status.OwnedClusterSPIFFEID == nil {
		t.Fatalf("ownership = pending %#v owned %#v, want confirmed", current.Status.PendingClusterSPIFFEID, current.Status.OwnedClusterSPIFFEID)
	}
	if current.Status.OwnedClusterSPIFFEID.UID != output.GetUID() {
		t.Fatalf("confirmed UID = %q, want %q", current.Status.OwnedClusterSPIFFEID.UID, output.GetUID())
	}
	if createCalls != 0 {
		t.Fatalf("ClusterSPIFFEID Create calls = %d, want 0", createCalls)
	}
}

func TestReserveClaimPersistsPendingObservedStatusAtomically(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-reserve-observed-status", "")
	binding.Status.RenderedClusterSPIFFEID = &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{Name: "stale-output"}
	reconciler := newConflictTestReconciler(t, binding)
	current := fetchBinding(t, ctx, reconciler.Client, binding.Name)

	if _, err := reconciler.reserveClusterSPIFFEIDClaim(ctx, current, "desired-output"); err != nil {
		t.Fatalf("reserve ClusterSPIFFEID claim: %v", err)
	}
	persisted := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if persisted.Status.PendingClusterSPIFFEID == nil || persisted.Status.PendingClusterSPIFFEID.Name != "desired-output" {
		t.Fatalf("pending ownership = %#v, want desired-output", persisted.Status.PendingClusterSPIFFEID)
	}
	if persisted.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v while claim is pending, want nil", persisted.Status.RenderedClusterSPIFFEID)
	}
	ready := meta.FindStatusCondition(persisted.Status.Conditions, conditionTypeReady)
	if ready == nil || ready.Status != metav1.ConditionUnknown || ready.Reason != conditionReasonInitializing {
		t.Fatalf("Ready while claim is pending = %#v, want Unknown/Initializing", ready)
	}
}

func TestCreateTimeoutConvergesThroughMatchingClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-create-timeout", ""))
	createCalls := 0
	returnTimeout := true
	timeoutErr := apierrors.NewTimeoutError("ambiguous ClusterSPIFFEID create", 1)
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			if object.GetObjectKind().GroupVersionKind() != clusterSPIFFEIDGVK {
				return cli.Create(ctx, object, opts...)
			}
			createCalls++
			if err := cli.Create(ctx, object, opts...); err != nil {
				return err
			}
			if returnTimeout {
				returnTimeout = false
				return timeoutErr
			}
			return nil
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest("binding-create-timeout")); !errors.Is(err, timeoutErr) {
		t.Fatalf("first Reconcile error = %v, want timeout", err)
	}
	pending := fetchBinding(t, ctx, wrapped, "binding-create-timeout")
	if pending.Status.PendingClusterSPIFFEID == nil {
		t.Fatal("pending ownership was not preserved after ambiguous create")
	}
	created := ownershipObjectByName(t, ctx, wrapped, pending.Status.PendingClusterSPIFFEID.Name)
	if got := spirecm.ClusterSPIFFEIDOwnershipClaim(created); got != pending.Status.PendingClusterSPIFFEID.ClaimID {
		t.Fatalf("created claim = %q, want %q", got, pending.Status.PendingClusterSPIFFEID.ClaimID)
	}

	reconcileBinding(t, ctx, reconciler, pending.Name)
	confirmed := fetchBinding(t, ctx, wrapped, pending.Name)
	if confirmed.Status.OwnedClusterSPIFFEID == nil || confirmed.Status.OwnedClusterSPIFFEID.UID != created.GetUID() {
		t.Fatalf("confirmed ownership = %#v, want UID %q", confirmed.Status.OwnedClusterSPIFFEID, created.GetUID())
	}
	if createCalls != 1 {
		t.Fatalf("ClusterSPIFFEID Create calls = %d, want 1", createCalls)
	}
}

func TestAmbiguousOwnershipPromotionPreservesPersistedOwnedUID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-ambiguous-promotion", ""))
	transitionErr := errors.New("ambiguous ownership promotion")
	created := false
	returnedError := false
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			err := cli.Create(ctx, object, opts...)
			if err == nil && object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				created = true
			}
			return err
		},
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			object client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			binding, ok := object.(*kleymv1alpha1.InferenceIdentityBinding)
			if created && !returnedError && ok && binding.Status.PendingClusterSPIFFEID == nil && binding.Status.OwnedClusterSPIFFEID != nil {
				if err := cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...); err != nil {
					return err
				}
				returnedError = true
				return transitionErr
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest("binding-ambiguous-promotion")); !errors.Is(err, transitionErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, transitionErr)
	}
	current := fetchBinding(t, ctx, wrapped, "binding-ambiguous-promotion")
	if current.Status.PendingClusterSPIFFEID != nil || current.Status.OwnedClusterSPIFFEID == nil || current.Status.OwnedClusterSPIFFEID.UID == "" {
		t.Fatalf("ownership after ambiguous promotion = pending %#v owned %#v, want confirmed UID only", current.Status.PendingClusterSPIFFEID, current.Status.OwnedClusterSPIFFEID)
	}
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
}

func TestAmbiguousOwnershipClearDoesNotResurrectOwnedUID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-ambiguous-clear", ""))
	reconcileBinding(t, ctx, base, "binding-ambiguous-clear")
	binding := fetchBinding(t, ctx, base.Client, "binding-ambiguous-clear")
	binding.Spec.IdentityBoundary.Variant = "invalid/variant"
	if err := base.Update(ctx, binding); err != nil {
		t.Fatalf("make binding invalid: %v", err)
	}

	transitionErr := errors.New("ambiguous ownership clear")
	returnedError := false
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			object client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			binding, ok := object.(*kleymv1alpha1.InferenceIdentityBinding)
			if !returnedError && ok && binding.Status.PendingClusterSPIFFEID == nil && binding.Status.OwnedClusterSPIFFEID == nil {
				if err := cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...); err != nil {
					return err
				}
				returnedError = true
				return transitionErr
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !errors.Is(err, transitionErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, transitionErr)
	}
	current := fetchBinding(t, ctx, wrapped, binding.Name)
	if current.Status.PendingClusterSPIFFEID != nil || current.Status.OwnedClusterSPIFFEID != nil {
		t.Fatalf("ownership after ambiguous clear = pending %#v owned %#v, want both nil", current.Status.PendingClusterSPIFFEID, current.Status.OwnedClusterSPIFFEID)
	}
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
}

func TestOrdinaryStatusPatchRetriesInterleavedOwnershipTransition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-interleaved-status", "")
	binding.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{Name: "output", ClaimID: "claim"}
	base := fakeClientForOwnershipTest(t, newControllerTestScheme(t), binding)
	desired := fetchBinding(t, ctx, base, binding.Name)
	desired.Status.TrustDomain = "updated.example"
	patchCalls := 0
	wrapped := interceptor.NewClient(base, interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			object client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			patchCalls++
			if patchCalls == 1 {
				interleaved := fetchBinding(t, ctx, cli, binding.Name)
				interleavedBase := interleaved.DeepCopy()
				interleaved.Status.PendingClusterSPIFFEID = nil
				interleaved.Status.OwnedClusterSPIFFEID = &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{Name: "output", UID: "owned-uid"}
				ownershipPatch := client.MergeFromWithOptions(interleavedBase, client.MergeFromWithOptimisticLock{})
				if err := cli.Status().Patch(ctx, interleaved, ownershipPatch); err != nil {
					return err
				}
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Client: wrapped}

	if err := reconciler.patchStatus(ctx, desired); err != nil {
		t.Fatalf("patch ordinary status after interleaving ownership: %v", err)
	}
	current := fetchBinding(t, ctx, wrapped, binding.Name)
	if patchCalls != 2 {
		t.Fatalf("ordinary status patch calls = %d, want conflict plus retry", patchCalls)
	}
	if current.Status.TrustDomain != "updated.example" {
		t.Fatalf("trustDomain = %q, want updated.example", current.Status.TrustDomain)
	}
	if current.Status.PendingClusterSPIFFEID != nil || current.Status.OwnedClusterSPIFFEID == nil || current.Status.OwnedClusterSPIFFEID.UID != "owned-uid" {
		t.Fatalf("ownership after interleaving patch = pending %#v owned %#v, want owned UID preserved", current.Status.PendingClusterSPIFFEID, current.Status.OwnedClusterSPIFFEID)
	}
}

func TestConfirmedUIDMismatchRefusesMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-confirmed-foreign", "")
	_, foreign := desiredOwnershipFixture(t, ctx, binding)
	foreign.SetUID("foreign-replacement-uid")
	foreign.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/workload"}
	wantSpec, _, _ := unstructured.NestedMap(foreign.Object, "spec")
	setConfirmedClusterSPIFFEID(binding, foreign.GetName(), "deleted-owned-uid")
	reconciler := newConflictTestReconciler(t, newTestPool(), binding, foreign)

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err == nil {
		t.Fatal("Reconcile error = nil, want UID mismatch refusal")
	}
	currentBinding := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if currentBinding.Status.OwnedClusterSPIFFEID != nil {
		t.Fatalf("stale confirmed ownership = %#v, want cleared", currentBinding.Status.OwnedClusterSPIFFEID)
	}
	observed := ownershipObjectByName(t, ctx, reconciler.Client, foreign.GetName())
	gotSpec, _, _ := unstructured.NestedMap(observed.Object, "spec")
	if observed.GetUID() != foreign.GetUID() || !reflect.DeepEqual(gotSpec, wantSpec) {
		t.Fatalf("foreign replacement was mutated: UID=%q spec=%#v", observed.GetUID(), gotSpec)
	}
}

func TestPendingClaimMismatchRefusesMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-pending-foreign", "")
	_, foreign := desiredOwnershipFixture(t, ctx, binding)
	foreign.SetUID("pending-foreign-uid")
	foreign.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/workload"}
	binding.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{
		Name:    foreign.GetName(),
		ClaimID: "expected-claim",
	}
	reconciler := newConflictTestReconciler(t, newTestPool(), binding, foreign)

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err == nil {
		t.Fatal("Reconcile error = nil, want pending claim mismatch refusal")
	}
	current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if current.Status.PendingClusterSPIFFEID != nil || current.Status.OwnedClusterSPIFFEID != nil {
		t.Fatalf("stale ownership = pending %#v owned %#v, want cleared", current.Status.PendingClusterSPIFFEID, current.Status.OwnedClusterSPIFFEID)
	}
	if observed := ownershipObjectByName(t, ctx, reconciler.Client, foreign.GetName()); observed.GetUID() != foreign.GetUID() {
		t.Fatalf("foreign UID = %q, want %q", observed.GetUID(), foreign.GetUID())
	}
}

func TestUnchangedConfirmedReconcileDoesNotPatchStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-unchanged-status", ""))
	reconcileBinding(t, ctx, base, "binding-unchanged-status")
	statusPatches := 0
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			object client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if subResourceName == "status" {
				statusPatches++
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	reconcileBinding(t, ctx, reconciler, "binding-unchanged-status")
	if statusPatches != 0 {
		t.Fatalf("unchanged reconcile status patches = %d, want 0", statusPatches)
	}
}

func TestChangedNameIgnoresForeignOldIncarnationAndCreatesReplacement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-name-change-foreign-old", "")
	_, foreignOld := desiredOwnershipFixture(t, ctx, binding)
	oldName := foreignOld.GetName()
	foreignOld.SetUID("foreign-old-uid")
	foreignOld.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/old"}
	setConfirmedClusterSPIFFEID(binding, oldName, "deleted-old-owned-uid")
	binding.Spec.ServiceAccountName = "inference-sa-v2"
	reconciler := newConflictTestReconciler(t, newTestPool(), binding, foreignOld)

	reconcileBinding(t, ctx, reconciler, binding.Name)
	current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if current.Status.OwnedClusterSPIFFEID == nil || current.Status.OwnedClusterSPIFFEID.Name == oldName {
		t.Fatalf("replacement ownership = %#v, want a new name", current.Status.OwnedClusterSPIFFEID)
	}
	if observed := ownershipObjectByName(t, ctx, reconciler.Client, oldName); observed.GetUID() != foreignOld.GetUID() {
		t.Fatalf("foreign old object UID = %q, want %q", observed.GetUID(), foreignOld.GetUID())
	}
	ownershipObjectByName(t, ctx, reconciler.Client, current.Status.OwnedClusterSPIFFEID.Name)
}

func TestValidationCleanupAndFinalizerPreserveUIDMismatchedReplacement(t *testing.T) {
	t.Parallel()

	t.Run("validation cleanup", func(t *testing.T) {
		ctx := context.Background()
		binding := newPoolOnlyBinding("binding-validation-foreign", "")
		_, foreign := desiredOwnershipFixture(t, ctx, binding)
		foreign.SetUID("validation-foreign-uid")
		setConfirmedClusterSPIFFEID(binding, foreign.GetName(), "deleted-owned-uid")
		binding.Spec.IdentityBoundary.Variant = "invalid/variant"
		reconciler := newConflictTestReconciler(t, binding, foreign)

		reconcileBinding(t, ctx, reconciler, binding.Name)
		current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
		if current.Status.OwnedClusterSPIFFEID != nil {
			t.Fatalf("stale confirmed ownership = %#v, want cleared", current.Status.OwnedClusterSPIFFEID)
		}
		if observed := ownershipObjectByName(t, ctx, reconciler.Client, foreign.GetName()); observed.GetUID() != foreign.GetUID() {
			t.Fatalf("foreign UID = %q, want %q", observed.GetUID(), foreign.GetUID())
		}
	})

	t.Run("finalizer", func(t *testing.T) {
		ctx := context.Background()
		binding := newPoolOnlyBinding("binding-finalizer-foreign", "")
		controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
		_, foreign := desiredOwnershipFixture(t, ctx, binding)
		foreign.SetUID("finalizer-foreign-uid")
		setConfirmedClusterSPIFFEID(binding, foreign.GetName(), "deleted-owned-uid")
		scheme := newControllerTestScheme(t)
		fakeClient := fakeClientForOwnershipTest(t, scheme, binding, foreign)
		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: fakeClient}

		current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
		if _, err := reconciler.reconcileDelete(ctx, current); err != nil {
			t.Fatalf("reconcileDelete returned error: %v", err)
		}
		current = fetchBinding(t, ctx, reconciler.Client, binding.Name)
		if controllerutil.ContainsFinalizer(current, inferenceIdentityBindingFinalizer) {
			t.Fatal("finalizer retained after the recorded incarnation was proven absent")
		}
		if observed := ownershipObjectByName(t, ctx, reconciler.Client, foreign.GetName()); observed.GetUID() != foreign.GetUID() {
			t.Fatalf("foreign UID = %q, want %q", observed.GetUID(), foreign.GetUID())
		}
	})
}

func TestCleanupDeleteUsesUIDPrecondition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-delete-precondition", "")
	owned := ownershipFixture("delete-precondition-output", "owned-uid", "")
	setConfirmedClusterSPIFFEID(binding, owned.GetName(), owned.GetUID())
	scheme := newControllerTestScheme(t)
	base := fakeClientForOwnershipTest(t, scheme, binding, owned)
	foreignUID := types.UID("foreign-replacement-uid")
	replaced := false
	wrapped := interceptor.NewClient(base, interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
			if object.GetObjectKind().GroupVersionKind() != clusterSPIFFEIDGVK || replaced {
				return cli.Delete(ctx, object, opts...)
			}
			replaced = true
			if err := cli.Delete(ctx, object); err != nil {
				return err
			}
			foreign := ownershipFixture(object.GetName(), foreignUID, "")
			if err := cli.Create(ctx, foreign); err != nil {
				return err
			}
			deleteOptions := (&client.DeleteOptions{}).ApplyOptions(opts)
			if deleteOptions.Preconditions == nil || deleteOptions.Preconditions.UID == nil || *deleteOptions.Preconditions.UID != owned.GetUID() {
				return errors.New("delete did not carry the confirmed UID precondition")
			}
			return apierrors.NewConflict(
				schema.GroupResource{Group: clusterSPIFFEIDGVK.Group, Resource: "clusterspiffeids"},
				object.GetName(),
				errors.New("UID precondition does not match the live object"),
			)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	current := fetchBinding(t, ctx, wrapped, binding.Name)
	if err := reconciler.cleanupManagedClusterSPIFFEIDs(ctx, current); !apierrors.IsConflict(err) {
		t.Fatalf("cleanup error = %v, want UID precondition conflict", err)
	}
	if observed := ownershipObjectByName(t, ctx, wrapped, owned.GetName()); observed.GetUID() != foreignUID {
		t.Fatalf("foreign replacement UID = %q, want %q", observed.GetUID(), foreignUID)
	}
	retained := fetchBinding(t, ctx, wrapped, binding.Name)
	if retained.Status.OwnedClusterSPIFFEID == nil || retained.Status.OwnedClusterSPIFFEID.UID != owned.GetUID() {
		t.Fatalf("confirmed ownership after delete uncertainty = %#v, want retained", retained.Status.OwnedClusterSPIFFEID)
	}
}

func ownershipFixture(name string, uid types.UID, claimID string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	object.SetName(name)
	object.SetUID(uid)
	if claimID != "" {
		spirecm.SetClusterSPIFFEIDOwnershipClaim(object, claimID)
	}
	return object
}

func desiredOwnershipFixture(
	t *testing.T,
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (renderedIdentity, *unstructured.Unstructured) {
	t.Helper()
	preflight := newConflictTestReconciler(t, newTestPool(), binding.DeepCopy())
	identity, err := preflight.renderIdentityForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("render identity: %v", err)
	}
	return identity, spirecm.DesiredClusterSPIFFEID(binding, identity, "")
}

func ownershipObjectByName(t *testing.T, ctx context.Context, cli client.Client, name string) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := cli.Get(ctx, types.NamespacedName{Name: name}, object); err != nil {
		t.Fatalf("get ClusterSPIFFEID %q: %v", name, err)
	}
	return object
}

func fakeClientForOwnershipTest(
	t *testing.T,
	scheme *runtime.Scheme,
	objects ...client.Object,
) client.WithWatch {
	t.Helper()
	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(objects...).
		Build()
	return withFakeClusterSPIFFEIDUIDs(base)
}
