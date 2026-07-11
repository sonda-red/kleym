package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestReconcileDeleteWaitsForManagedClusterSPIFFEIDsToDisappear(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	binding := newPoolOnlyBinding("binding-delete", "")
	controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
	binding.SetDeletionTimestamp(&metav1.Time{Time: metav1.Now().Time})

	managed := newManagedClusterSPIFFEIDForBinding(binding, "binding-delete-child")
	managed.SetFinalizers([]string{"test.finalizer/hold"})
	binding.Status.OwnedClusterSPIFFEIDName = managed.GetName()

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(binding, managed).
			Build(),
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

func TestCleanupDoesNotDeleteForeignOutputWithManagedLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	binding := newPoolOnlyBinding("binding-cleanup-foreign", "")
	recorded := newManagedClusterSPIFFEIDForBinding(binding, "recorded-managed-output")
	foreign := newManagedClusterSPIFFEIDForBinding(binding, "spoofed-managed-output")
	binding.Status.OwnedClusterSPIFFEIDName = recorded.GetName()
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding, recorded, foreign).Build(),
	}

	if err := reconciler.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
		t.Fatalf("cleanupManagedClusterSPIFFEIDs returned error: %v", err)
	}
	observed := &unstructured.Unstructured{}
	observed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: recorded.GetName()}, observed); !apierrors.IsNotFound(err) {
		t.Fatalf("recorded output lookup error = %v, want NotFound", err)
	}
	observed = &unstructured.Unstructured{}
	observed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: foreign.GetName()}, observed); err != nil {
		t.Fatalf("foreign matching-label output was deleted: %v", err)
	}
}

func TestChangedOutputNameWaitsForRecordedOutputAbsence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	reconciler := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-name-change", ""))
	reconcileBinding(t, ctx, reconciler, "binding-name-change")
	binding := fetchBinding(t, ctx, reconciler.Client, "binding-name-change")
	oldName := binding.Status.OwnedClusterSPIFFEIDName
	oldOutput := managedOutputForBinding(t, ctx, reconciler.Client, binding)
	oldOutput.SetFinalizers([]string{"test.finalizer/hold"})
	if err := reconciler.Update(ctx, oldOutput); err != nil {
		t.Fatalf("hold old output deletion: %v", err)
	}
	if err := reconciler.Get(ctx, types.NamespacedName{Name: oldName}, oldOutput); err != nil {
		t.Fatalf("refetch held old output: %v", err)
	}
	if len(oldOutput.GetFinalizers()) == 0 {
		t.Fatalf("old output finalizers = %v, want deletion hold", oldOutput.GetFinalizers())
	}

	binding.Spec.ServiceAccountName = "inference-sa-v2"
	if err := reconciler.Update(ctx, binding); err != nil {
		t.Fatalf("change identity: %v", err)
	}
	identity, err := reconciler.renderIdentityForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("render changed identity: %v", err)
	}
	newName := spirecm.DesiredClusterSPIFFEID(binding, identity, "").GetName()
	if newName == oldName {
		t.Fatalf("changed identity retained output name %q", oldName)
	}

	result := reconcileBinding(t, ctx, reconciler, binding.Name)
	if result.RequeueAfter != deleteVerificationRequeueAfter {
		t.Fatalf("name-change requeueAfter = %s, want %s", result.RequeueAfter, deleteVerificationRequeueAfter)
	}
	pending := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if pending.Status.OwnedClusterSPIFFEIDName != oldName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q while old output remains, want %q", pending.Status.OwnedClusterSPIFFEIDName, oldName)
	}
	if pending.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v while replacement is pending, want nil", pending.Status.RenderedClusterSPIFFEID)
	}
	ready := meta.FindStatusCondition(pending.Status.Conditions, conditionTypeReady)
	if ready == nil || ready.Status != metav1.ConditionUnknown || ready.Reason != conditionReasonInitializing {
		t.Fatalf("Ready while replacement is pending = %#v, want Unknown/Initializing", ready)
	}
	newOutput := &unstructured.Unstructured{}
	newOutput.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: newName}, newOutput); !apierrors.IsNotFound(err) {
		t.Fatalf("replacement output lookup error = %v, want NotFound before old output absence", err)
	}

	oldOutput = &unstructured.Unstructured{}
	oldOutput.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: oldName}, oldOutput); err != nil {
		t.Fatalf("get terminating old output: %v", err)
	}
	if oldOutput.GetDeletionTimestamp() == nil {
		t.Fatal("old output deletion was not requested")
	}
	oldOutput.SetFinalizers(nil)
	if err := reconciler.Update(ctx, oldOutput); err != nil {
		t.Fatalf("release old output deletion: %v", err)
	}

	reconcileBinding(t, ctx, reconciler, binding.Name)
	replaced := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if replaced.Status.OwnedClusterSPIFFEIDName != newName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after replacement, want %q", replaced.Status.OwnedClusterSPIFFEIDName, newName)
	}
	assertSuccessConditionSet(t, replaced)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: newName}, newOutput); err != nil {
		t.Fatalf("get replacement output: %v", err)
	}
	if err := reconciler.Get(ctx, types.NamespacedName{Name: oldName}, oldOutput); !apierrors.IsNotFound(err) {
		t.Fatalf("old output lookup error = %v, want NotFound after replacement", err)
	}

	if err := reconciler.Delete(ctx, replaced); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	reconcileBinding(t, ctx, reconciler, binding.Name)
	terminating := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, bindingRequest(binding.Name).NamespacedName, terminating); !apierrors.IsNotFound(err) {
		t.Fatalf("binding lookup error = %v after finalizer cleanup, want NotFound", err)
	}
	if err := reconciler.Get(ctx, types.NamespacedName{Name: newName}, newOutput); !apierrors.IsNotFound(err) {
		t.Fatalf("replacement output lookup error = %v after finalizer cleanup, want NotFound", err)
	}
}

func TestChangedOutputNameDeleteFailureRetainsRecordedOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-name-change-failure", ""))
	reconcileBinding(t, ctx, base, "binding-name-change-failure")
	binding := fetchBinding(t, ctx, base.Client, "binding-name-change-failure")
	oldName := binding.Status.OwnedClusterSPIFFEIDName
	binding.Spec.ServiceAccountName = "inference-sa-v2"
	if err := base.Update(ctx, binding); err != nil {
		t.Fatalf("change identity: %v", err)
	}
	identity, err := base.renderIdentityForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("render changed identity: %v", err)
	}
	newName := spirecm.DesiredClusterSPIFFEID(binding, identity, "").GetName()

	deleteErr := errors.New("delete previous output failed")
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
			if object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK && object.GetName() == oldName {
				return deleteErr
			}
			return cli.Delete(ctx, object, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}
	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !errors.Is(err, deleteErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, deleteErr)
	}
	failed := fetchBinding(t, ctx, wrapped, binding.Name)
	if failed.Status.OwnedClusterSPIFFEIDName != oldName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after delete failure, want %q", failed.Status.OwnedClusterSPIFFEIDName, oldName)
	}
	assertPrimaryFailureCondition(t, failed, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	newOutput := &unstructured.Unstructured{}
	newOutput.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := wrapped.Get(ctx, types.NamespacedName{Name: newName}, newOutput); !apierrors.IsNotFound(err) {
		t.Fatalf("replacement output lookup error = %v, want NotFound after old-output delete failure", err)
	}
}

func TestChangedOutputNameStatusFailureRetainsReplacementClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-name-change-status-failure", ""))
	reconcileBinding(t, ctx, base, "binding-name-change-status-failure")
	binding := fetchBinding(t, ctx, base.Client, "binding-name-change-status-failure")
	oldName := binding.Status.OwnedClusterSPIFFEIDName
	binding.Spec.ServiceAccountName = "inference-sa-v2"
	if err := base.Update(ctx, binding); err != nil {
		t.Fatalf("change identity: %v", err)
	}
	identity, err := base.renderIdentityForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("render changed identity: %v", err)
	}
	newName := spirecm.DesiredClusterSPIFFEID(binding, identity, "").GetName()

	statusErr := errors.New("patch replacement success status failed")
	replacementCreated := false
	failedSuccessPatch := false
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			err := cli.Create(ctx, object, opts...)
			if err == nil && object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK && object.GetName() == newName {
				replacementCreated = true
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
			if replacementCreated && !failedSuccessPatch && subResourceName == "status" {
				failedSuccessPatch = true
				return statusErr
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !errors.Is(err, statusErr) {
		t.Fatalf("replacement Reconcile error = %v, want %v", err, statusErr)
	}
	pending := fetchBinding(t, ctx, wrapped, binding.Name)
	if pending.Status.PendingClusterSPIFFEIDName != newName {
		t.Fatalf("pendingClusterSPIFFEIDName = %q, want replacement %q", pending.Status.PendingClusterSPIFFEIDName, newName)
	}
	if pending.Status.OwnedClusterSPIFFEIDName != "" {
		t.Fatalf("ownedClusterSPIFFEIDName = %q before replacement confirmation, want empty", pending.Status.OwnedClusterSPIFFEIDName)
	}
	if pending.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after replacement status failure, want nil", pending.Status.RenderedClusterSPIFFEID)
	}
	oldOutput := &unstructured.Unstructured{}
	oldOutput.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := wrapped.Get(ctx, types.NamespacedName{Name: oldName}, oldOutput); !apierrors.IsNotFound(err) {
		t.Fatalf("old output lookup error = %v, want NotFound", err)
	}
	newOutput := &unstructured.Unstructured{}
	newOutput.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := wrapped.Get(ctx, types.NamespacedName{Name: newName}, newOutput); err != nil {
		t.Fatalf("replacement output missing after status failure: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err != nil {
		t.Fatalf("replacement retry Reconcile returned error: %v", err)
	}
	confirmed := fetchBinding(t, ctx, wrapped, binding.Name)
	assertSuccessConditionSet(t, confirmed)
	if confirmed.Status.PendingClusterSPIFFEIDName != "" || confirmed.Status.OwnedClusterSPIFFEIDName != newName {
		t.Fatalf("replacement ownership = pending %q owned %q, want empty/%q", confirmed.Status.PendingClusterSPIFFEIDName, confirmed.Status.OwnedClusterSPIFFEIDName, newName)
	}
}

func TestRecordedOutputUpdateFailureRetainsOwnershipAndRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-update-retry", ""))
	reconcileBinding(t, ctx, base, "binding-update-retry")
	binding := fetchBinding(t, ctx, base.Client, "binding-update-retry")
	recordedName := binding.Status.OwnedClusterSPIFFEIDName
	if recordedName == "" {
		t.Fatal("ownedClusterSPIFFEIDName was not recorded after create")
	}

	drifted := &unstructured.Unstructured{}
	drifted.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := base.Get(ctx, types.NamespacedName{Name: recordedName}, drifted); err != nil {
		t.Fatalf("get managed output: %v", err)
	}
	drifted.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://drifted.example/workload"}
	if err := base.Update(ctx, drifted); err != nil {
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
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !errors.Is(err, updateErr) {
		t.Fatalf("first Reconcile error = %v, want %v", err, updateErr)
	}
	failed := fetchBinding(t, ctx, wrapped, binding.Name)
	if failed.Status.OwnedClusterSPIFFEIDName != recordedName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after failure, want %q", failed.Status.OwnedClusterSPIFFEIDName, recordedName)
	}
	if failed.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after failure, want cleared", failed.Status.RenderedClusterSPIFFEID)
	}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err != nil {
		t.Fatalf("retry Reconcile returned error: %v", err)
	}
	retried := fetchBinding(t, ctx, wrapped, binding.Name)
	assertSuccessConditionSet(t, retried)
	if retried.Status.OwnedClusterSPIFFEIDName != recordedName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after retry, want %q", retried.Status.OwnedClusterSPIFFEIDName, recordedName)
	}
	if retried.Status.RenderedClusterSPIFFEID == nil {
		t.Fatal("renderedClusterSPIFFEID was not restored after retry")
	}
}

func TestCreateStatusPatchFailureRetainsPendingClaimAndRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-create-status-retry", ""))
	statusErr := errors.New("patch success status failed")
	created := false
	failedSuccessPatch := false
	createCalls := 0
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			err := cli.Create(ctx, object, opts...)
			if object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				createCalls++
				created = err == nil
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
			if created && !failedSuccessPatch && subResourceName == "status" {
				failedSuccessPatch = true
				return statusErr
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest("binding-create-status-retry")); !errors.Is(err, statusErr) {
		t.Fatalf("first Reconcile error = %v, want %v", err, statusErr)
	}
	pending := fetchBinding(t, ctx, wrapped, "binding-create-status-retry")
	if pending.Status.PendingClusterSPIFFEIDName == "" {
		t.Fatal("pendingClusterSPIFFEIDName was not persisted before Create")
	}
	if pending.Status.OwnedClusterSPIFFEIDName != "" {
		t.Fatalf("ownedClusterSPIFFEIDName = %q before confirmation, want empty", pending.Status.OwnedClusterSPIFFEIDName)
	}
	if pending.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after status failure, want nil", pending.Status.RenderedClusterSPIFFEID)
	}
	assertClusterSPIFFEIDCount(t, ctx, wrapped, 1)

	if _, err := reconciler.Reconcile(ctx, bindingRequest(pending.Name)); err != nil {
		t.Fatalf("retry Reconcile returned error: %v", err)
	}
	confirmed := fetchBinding(t, ctx, wrapped, pending.Name)
	assertSuccessConditionSet(t, confirmed)
	if confirmed.Status.PendingClusterSPIFFEIDName != "" {
		t.Fatalf("pendingClusterSPIFFEIDName = %q after confirmation, want empty", confirmed.Status.PendingClusterSPIFFEIDName)
	}
	if confirmed.Status.OwnedClusterSPIFFEIDName == "" {
		t.Fatal("ownedClusterSPIFFEIDName was not confirmed on retry")
	}
	if createCalls != 1 {
		t.Fatalf("ClusterSPIFFEID Create calls = %d, want 1", createCalls)
	}
}

func TestReservationStatusPatchFailureDoesNotCreateOrPersistClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-reservation-status-failure", ""))
	statusErr := errors.New("patch pending claim failed")
	failedReservationPatch := false
	createCalls := 0
	wrapped := interceptor.NewClient(base.Client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			if object.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				createCalls++
			}
			return cli.Create(ctx, object, opts...)
		},
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			object client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if !failedReservationPatch && subResourceName == "status" {
				failedReservationPatch = true
				return statusErr
			}
			return cli.SubResource(subResourceName).Patch(ctx, object, patch, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}

	if _, err := reconciler.Reconcile(ctx, bindingRequest("binding-reservation-status-failure")); !errors.Is(err, statusErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, statusErr)
	}
	failed := fetchBinding(t, ctx, wrapped, "binding-reservation-status-failure")
	if failed.Status.PendingClusterSPIFFEIDName != "" || failed.Status.OwnedClusterSPIFFEIDName != "" {
		t.Fatalf("ownership status = pending %q owned %q, want both empty", failed.Status.PendingClusterSPIFFEIDName, failed.Status.OwnedClusterSPIFFEIDName)
	}
	if failed.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after reservation failure, want nil", failed.Status.RenderedClusterSPIFFEID)
	}
	assertPrimaryFailureCondition(t, failed, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	if createCalls != 0 {
		t.Fatalf("ClusterSPIFFEID Create calls = %d, want 0", createCalls)
	}
}

func TestUnclaimedPreExistingDeterministicNameIsForeign(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-foreign-deterministic-name", "")
	reconciler := newConflictTestReconciler(t, newTestPool(), binding)
	identity, err := reconciler.renderIdentityForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("render identity: %v", err)
	}
	foreign := spirecm.DesiredClusterSPIFFEID(binding, identity, "")
	foreign.Object["spec"] = map[string]any{"spiffeIDTemplate": "spiffe://foreign.example/workload"}
	if err := reconciler.Create(ctx, foreign); err != nil {
		t.Fatalf("create foreign deterministic-name output: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err == nil {
		t.Fatal("Reconcile error = nil, want refusal of unclaimed pre-existing output")
	}
	failed := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if failed.Status.PendingClusterSPIFFEIDName != "" || failed.Status.OwnedClusterSPIFFEIDName != "" {
		t.Fatalf("ownership status = pending %q owned %q, want both empty", failed.Status.PendingClusterSPIFFEIDName, failed.Status.OwnedClusterSPIFFEIDName)
	}
	if failed.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after foreign-name refusal, want nil", failed.Status.RenderedClusterSPIFFEID)
	}
	observed := &unstructured.Unstructured{}
	observed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := reconciler.Get(ctx, types.NamespacedName{Name: foreign.GetName()}, observed); err != nil {
		t.Fatalf("foreign deterministic-name output was removed: %v", err)
	}
	spiffeID, _, err := unstructured.NestedString(observed.Object, "spec", "spiffeIDTemplate")
	if err != nil || spiffeID != "spiffe://foreign.example/workload" {
		t.Fatalf("foreign output spiffeIDTemplate = %q, error %v", spiffeID, err)
	}
}

func TestFinalizationNoMatchPreservesOwnershipAndFinalizer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-finalize-no-match", "")
	controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
	binding.Status.OwnedClusterSPIFFEIDName = "recorded-output"
	binding.Status.RenderedClusterSPIFFEID = &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{Name: "recorded-output"}
	scheme := newControllerTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(binding).
		Build()
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: noMatchClient{Client: base, getNoMatchGVK: clusterSPIFFEIDGVK},
	}

	fetched := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if _, err := reconciler.reconcileDelete(ctx, fetched); !meta.IsNoMatchError(err) {
		t.Fatalf("reconcileDelete error = %v, want NoMatch", err)
	}
	retained := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if retained.Status.OwnedClusterSPIFFEIDName != "recorded-output" {
		t.Fatalf("ownedClusterSPIFFEIDName = %q, want retained", retained.Status.OwnedClusterSPIFFEIDName)
	}
	if retained.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after NoMatch, want nil", retained.Status.RenderedClusterSPIFFEID)
	}
	if !controllerutil.ContainsFinalizer(retained, inferenceIdentityBindingFinalizer) {
		t.Fatal("binding finalizer was removed without confirmed output absence")
	}
}

func TestValidationCleanupNoMatchPreservesOwnershipAndRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	binding := newPoolOnlyBinding("binding-validation-no-match", "")
	controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
	binding.Status.OwnedClusterSPIFFEIDName = "recorded-output"
	binding.Status.RenderedClusterSPIFFEID = &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{Name: "recorded-output"}
	binding.Spec.IdentityBoundary.LabelKey = "example.com/not-reserved"
	scheme := newControllerTestScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(binding).
		Build()
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: noMatchClient{Client: base, getNoMatchGVK: clusterSPIFFEIDGVK},
	}

	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !meta.IsNoMatchError(err) {
		t.Fatalf("Reconcile error = %v, want NoMatch retry", err)
	}
	failed := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	if failed.Status.OwnedClusterSPIFFEIDName != "recorded-output" {
		t.Fatalf("ownedClusterSPIFFEIDName = %q, want retained", failed.Status.OwnedClusterSPIFFEIDName)
	}
	if failed.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v after NoMatch, want nil", failed.Status.RenderedClusterSPIFFEID)
	}
	assertPrimaryFailureCondition(t, failed, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
}

func TestFinalizerCleanupUsesOwnershipRetainedAfterUpdateFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := newConflictTestReconciler(t, newTestPool(), newPoolOnlyBinding("binding-failure-finalize", ""))
	reconcileBinding(t, ctx, base, "binding-failure-finalize")
	binding := fetchBinding(t, ctx, base.Client, "binding-failure-finalize")
	recordedName := binding.Status.OwnedClusterSPIFFEIDName
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
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: wrapped}
	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); !errors.Is(err, updateErr) {
		t.Fatalf("failure Reconcile error = %v, want %v", err, updateErr)
	}
	failed := fetchBinding(t, ctx, wrapped, binding.Name)
	if failed.Status.OwnedClusterSPIFFEIDName != recordedName {
		t.Fatalf("ownedClusterSPIFFEIDName = %q after failure, want %q", failed.Status.OwnedClusterSPIFFEIDName, recordedName)
	}
	if err := wrapped.Delete(ctx, failed); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, bindingRequest(binding.Name)); err != nil {
		t.Fatalf("finalizer Reconcile returned error: %v", err)
	}
	observed := &unstructured.Unstructured{}
	observed.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := wrapped.Get(ctx, types.NamespacedName{Name: recordedName}, observed); !apierrors.IsNotFound(err) {
		t.Fatalf("managed output lookup error = %v, want NotFound", err)
	}
	cleared := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := wrapped.Get(ctx, bindingRequest(binding.Name).NamespacedName, cleared); !apierrors.IsNotFound(err) {
		t.Fatalf("binding lookup error = %v after finalizer cleanup, want NotFound", err)
	}
}

func TestReconcileCorrectsClusterSPIFFEIDDriftOnResync(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	binding := newPoolOnlyBinding("binding-drift", "")

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(
				newTestPool(),
				binding,
			).
			Build(),
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
	desired := spirecm.DesiredClusterSPIFFEID(currentBinding, identity, "")

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
		"spiffeIDTemplate":          "spiffe://drifted.example/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill",
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

	if !spirecm.ClusterSPIFFEIDInSync(current, desired) {
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
	managed.SetLabels(spirecm.ManagedClusterSPIFFEIDLabels(binding))
	managed.Object["spec"] = map[string]any{
		"spiffeIDTemplate": "spiffe://example.test/ns/default/sa/inference-sa/inference/pool/example/variant/prefill",
	}
	return managed
}
