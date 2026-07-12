package controller

import (
	"context"
	stderrors "errors"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/identity"
)

func TestReconcileConditionTaxonomySuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	binding := newPoolOnlyBinding("binding-taxonomy-success", "")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(newTestPool(), binding).
		Build()
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: withFakeClusterSPIFFEIDUIDs(fakeClient),
	}

	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %#v, want empty result", result)
	}

	current := fetchBinding(t, ctx, reconciler.Client, binding.Name)
	assertSuccessConditionSet(t, current)
	if len(current.Status.ComputedSpiffeIDs) != 1 {
		t.Fatalf("computedSpiffeIDs = %d, want 1", len(current.Status.ComputedSpiffeIDs))
	}
	if len(current.Status.RenderedSelectors) != 1 {
		t.Fatalf("renderedSelectors = %d, want 1", len(current.Status.RenderedSelectors))
	}
	if current.Status.RenderedClusterSPIFFEID == nil {
		t.Fatalf("renderedClusterSPIFFEID was not populated")
	}
	if current.Status.RenderedClusterSPIFFEID.SpiffeID != current.Status.ComputedSpiffeIDs[0].SpiffeID {
		t.Fatalf(
			"renderedClusterSPIFFEID.spiffeID = %q, want %q",
			current.Status.RenderedClusterSPIFFEID.SpiffeID,
			current.Status.ComputedSpiffeIDs[0].SpiffeID,
		)
	}
	if current.Status.RenderedClusterSPIFFEID.Name == "" {
		t.Fatalf("renderedClusterSPIFFEID.name was not populated")
	}
	if current.Status.RenderedClusterSPIFFEID.SelectorFingerprint == "" {
		t.Fatalf("renderedClusterSPIFFEID.selectorFingerprint was not populated")
	}
	if current.Status.OwnedClusterSPIFFEID == nil || current.Status.OwnedClusterSPIFFEID.Name != current.Status.RenderedClusterSPIFFEID.Name || current.Status.OwnedClusterSPIFFEID.UID == "" {
		t.Fatalf("ownedClusterSPIFFEID = %#v, want rendered name %q and a UID", current.Status.OwnedClusterSPIFFEID, current.Status.RenderedClusterSPIFFEID.Name)
	}
}

func TestReconcileManagedOutputApplyFailureSetsFailureStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	binding := newPoolOnlyBinding("binding-managed-output-apply-failure", "")
	binding.Status = kleymv1alpha1.InferenceIdentityBindingStatus{
		ComputedSpiffeIDs: []kleymv1alpha1.ComputedSpiffeIDStatus{{
			SpiffeID: "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
		}},
		RenderedSelectors: []kleymv1alpha1.RenderedSelectorStatus{{
			SpiffeID:  "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
			Selectors: []string{"k8s:ns:default", "k8s:sa:old"},
		}},
		RenderedClusterSPIFFEID: &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{
			Name:                "stale",
			SpiffeID:            "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
			SelectorFingerprint: "sha256:stale",
		},
		Conditions: []metav1.Condition{{
			Type:    conditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  conditionReasonReconciled,
			Message: "stale success",
		}},
	}

	applyErr := stderrors.New("create managed ClusterSPIFFEID failed")
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(newTestPool(), binding).
		Build()
	k8sClient := interceptor.NewClient(baseClient, interceptor.Funcs{
		Create: func(
			ctx context.Context,
			k8sClient client.WithWatch,
			obj client.Object,
			opts ...client.CreateOption,
		) error {
			if obj.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				return applyErr
			}
			return k8sClient.Create(ctx, obj, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: k8sClient,
	}

	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if !stderrors.Is(err, applyErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, applyErr)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %#v, want empty result", result)
	}

	current := fetchBinding(t, ctx, k8sClient, binding.Name)
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	if len(current.Status.ComputedSpiffeIDs) != 0 {
		t.Fatalf("computedSpiffeIDs = %d, want cleared on managed-output apply failure", len(current.Status.ComputedSpiffeIDs))
	}
	if len(current.Status.RenderedSelectors) != 0 {
		t.Fatalf("renderedSelectors = %d, want cleared on managed-output apply failure", len(current.Status.RenderedSelectors))
	}
	if current.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v, want cleared on managed-output apply failure", current.Status.RenderedClusterSPIFFEID)
	}
	if current.Status.OwnedClusterSPIFFEID != nil {
		t.Fatalf("ownedClusterSPIFFEID = %#v, want nil after failed create", current.Status.OwnedClusterSPIFFEID)
	}
	if current.Status.PendingClusterSPIFFEID == nil || current.Status.PendingClusterSPIFFEID.ClaimID == "" {
		t.Fatal("pendingClusterSPIFFEID claim is empty after ambiguous create failure")
	}
}

func TestReconcileFailureCleanupApplyFailureSetsManagedOutputFailureStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	binding := newPoolOnlyBinding("binding-managed-output-cleanup-failure", "")
	binding.Spec.PoolRef.Name = "missing-pool"
	binding.Status = kleymv1alpha1.InferenceIdentityBindingStatus{
		OwnedClusterSPIFFEID: &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{Name: "stale", UID: "stale-uid"},
		ComputedSpiffeIDs: []kleymv1alpha1.ComputedSpiffeIDStatus{{
			SpiffeID: "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
		}},
		RenderedSelectors: []kleymv1alpha1.RenderedSelectorStatus{{
			SpiffeID:  "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
			Selectors: []string{"k8s:ns:default", "k8s:sa:old"},
		}},
		RenderedClusterSPIFFEID: &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{
			Name:                "stale",
			SpiffeID:            "spiffe://stale.example/ns/default/sa/inference-sa/inference/pool/old/variant/prefill",
			SelectorFingerprint: "sha256:stale",
		},
		Conditions: []metav1.Condition{{
			Type:    conditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  conditionReasonReconciled,
			Message: "stale success",
		}},
	}

	cleanupErr := stderrors.New("list managed ClusterSPIFFEIDs for cleanup failed")
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(binding).
		Build()
	k8sClient := interceptor.NewClient(baseClient, interceptor.Funcs{
		Get: func(
			ctx context.Context,
			k8sClient client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			if obj.GetObjectKind().GroupVersionKind() == clusterSPIFFEIDGVK {
				return cleanupErr
			}
			return k8sClient.Get(ctx, key, obj, opts...)
		},
	})
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: k8sClient,
	}

	result, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if !stderrors.Is(err, cleanupErr) {
		t.Fatalf("Reconcile error = %v, want %v", err, cleanupErr)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %#v, want empty result", result)
	}

	current := fetchBinding(t, ctx, k8sClient, binding.Name)
	assertPrimaryFailureCondition(t, current, conditionTypeRenderFailure, conditionReasonManagedOutputApplyFailed)
	if len(current.Status.ComputedSpiffeIDs) != 0 {
		t.Fatalf("computedSpiffeIDs = %d, want cleared on cleanup failure", len(current.Status.ComputedSpiffeIDs))
	}
	if len(current.Status.RenderedSelectors) != 0 {
		t.Fatalf("renderedSelectors = %d, want cleared on cleanup failure", len(current.Status.RenderedSelectors))
	}
	if current.Status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v, want cleared on cleanup failure", current.Status.RenderedClusterSPIFFEID)
	}
	if confirmedClusterSPIFFEIDName(current) != "stale" || current.Status.OwnedClusterSPIFFEID.UID != "stale-uid" {
		t.Fatalf("ownedClusterSPIFFEID = %#v, want retained after cleanup failure", current.Status.OwnedClusterSPIFFEID)
	}
}

func TestReconcileConditionTaxonomyFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		binding       *kleymv1alpha1.InferenceIdentityBinding
		config        *OperatorConfig
		objects       []client.Object
		wrapClient    func(client.Client) client.Client
		wantResult    ctrl.Result
		wantCondition string
		wantReason    string
	}{
		"invalid-pool-ref": {
			binding: func() *kleymv1alpha1.InferenceIdentityBinding {
				binding := newPoolOnlyBinding("binding-invalid-pool-ref-taxonomy", "")
				binding.Spec.PoolRef.Name = " "
				return binding
			}(),
			wantCondition: conditionTypeInvalidRef,
			wantReason:    conditionReasonInvalidPoolRef,
		},
		"missing-pool": {
			binding:       newPoolOnlyBinding("binding-missing-pool-taxonomy", ""),
			wantCondition: conditionTypeInvalidRef,
			wantReason:    gaie.ReasonTargetPoolNotFound,
		},
		"invalid-pool-selector": {
			binding: newPoolOnlyBinding("binding-invalid-selector-taxonomy", ""),
			objects: []client.Object{
				testPoolWithSelector("pool-a", map[string]any{"matchLabels": map[string]any{"app": []any{"model-server"}}}),
			},
			wantCondition: conditionTypeUnsafeSelector,
			wantReason:    identity.ReasonInvalidPoolSelector,
		},
		"invalid-service-account": {
			binding:       newPoolBindingWithServiceAccount("binding-invalid-sa-taxonomy", "Invalid_ServiceAccount"),
			objects:       []client.Object{newTestPool()},
			wantCondition: conditionTypeRenderFailure,
			wantReason:    identity.ReasonInvalidServiceAccountName,
		},
		"invalid-identity-boundary": {
			binding: func() *kleymv1alpha1.InferenceIdentityBinding {
				binding := newPoolOnlyBinding("binding-invalid-boundary-taxonomy", "")
				binding.Spec.IdentityBoundary.Variant = "invalid/variant"
				return binding
			}(),
			objects:       []client.Object{newTestPool()},
			wantCondition: conditionTypeUnsafeSelector,
			wantReason:    identity.ReasonInvalidIdentityBoundary,
		},
		"missing-trust-domain": {
			binding:       newPoolOnlyBinding("binding-missing-trust-domain-taxonomy", ""),
			config:        &OperatorConfig{},
			objects:       []client.Object{newTestPool()},
			wantCondition: conditionTypeRenderFailure,
			wantReason:    identity.ReasonMissingTrustDomain,
		},
		"missing-inferencepool-crd": {
			binding: newPoolOnlyBinding("binding-missing-pool-crd-taxonomy", ""),
			wrapClient: func(base client.Client) client.Client {
				return noMatchClient{
					Client:        base,
					getNoMatchGVK: inferencePoolGVKs[0],
				}
			},
			wantResult:    ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter},
			wantCondition: conditionTypeInvalidRef,
			wantReason:    gaie.ReasonInferencePoolCRDMissing,
		},
		"missing-clusterspiffeid-crd": {
			binding: newPoolOnlyBinding("binding-missing-clusterspiffeid-crd-taxonomy", ""),
			objects: []client.Object{
				newTestPool(),
			},
			wrapClient: func(base client.Client) client.Client {
				return noMatchClient{
					Client:        base,
					getNoMatchGVK: clusterSPIFFEIDGVK,
				}
			},
			wantResult:    ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter},
			wantCondition: conditionTypeRenderFailure,
			wantReason:    conditionReasonClusterSPIFFEIDCRDMissing,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			objects := append([]client.Object{tc.binding}, tc.objects...)
			baseClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
				WithObjects(objects...).
				Build()
			k8sClient := client.Client(baseClient)
			if tc.wrapClient != nil {
				k8sClient = tc.wrapClient(baseClient)
			}
			config := testOperatorConfig()
			if tc.config != nil {
				config = *tc.config
			}
			reconciler := &InferenceIdentityBindingReconciler{
				Config: config,
				Client: k8sClient,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: tc.binding.Name},
			})
			if err != nil {
				t.Fatalf("Reconcile returned error: %v", err)
			}
			if result != tc.wantResult {
				t.Fatalf("result = %#v, want %#v", result, tc.wantResult)
			}

			current := fetchBinding(t, ctx, k8sClient, tc.binding.Name)
			assertPrimaryFailureCondition(t, current, tc.wantCondition, tc.wantReason)
			if len(current.Status.ComputedSpiffeIDs) != 0 {
				t.Fatalf("computedSpiffeIDs = %d, want cleared on failure", len(current.Status.ComputedSpiffeIDs))
			}
			if len(current.Status.RenderedSelectors) != 0 {
				t.Fatalf("renderedSelectors = %d, want cleared on failure", len(current.Status.RenderedSelectors))
			}
			if current.Status.RenderedClusterSPIFFEID != nil {
				t.Fatalf("renderedClusterSPIFFEID = %#v, want cleared on failure", current.Status.RenderedClusterSPIFFEID)
			}
		})
	}
}

func TestConditionTaxonomyAllowedReasonStrings(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		got  string
		want string
	}{
		"ready-reconciled":             {got: conditionReasonReconciled, want: "Reconciled"},
		"inactive-resolved":            {got: conditionReasonResolved, want: "Resolved"},
		"unevaluated-initializing":     {got: conditionReasonInitializing, want: "Initializing"},
		"invalid-pool-ref":             {got: conditionReasonInvalidPoolRef, want: "InvalidPoolRef"},
		"target-pool-not-found":        {got: gaie.ReasonTargetPoolNotFound, want: "TargetPoolNotFound"},
		"inferencepool-crd-missing":    {got: gaie.ReasonInferencePoolCRDMissing, want: "InferencePoolCRDMissing"},
		"invalid-pool-selector":        {got: identity.ReasonInvalidPoolSelector, want: "InvalidPoolSelector"},
		"invalid-identity-boundary":    {got: identity.ReasonInvalidIdentityBoundary, want: "InvalidIdentityBoundary"},
		"unsafe-selector":              {got: identity.ReasonUnsafeSelector, want: "UnsafeSelector"},
		"missing-trust-domain":         {got: identity.ReasonMissingTrustDomain, want: "MissingTrustDomain"},
		"invalid-service-account-name": {got: identity.ReasonInvalidServiceAccountName, want: "InvalidServiceAccountName"},
		"invalid-spiffe-id":            {got: identity.ReasonInvalidSPIFFEID, want: "InvalidSPIFFEID"},
		"variant-conflict":             {got: conditionReasonVariantConflict, want: "VariantConflict"},
		"duplicate-spiffe-id":          {got: conditionReasonDuplicateSPIFFEID, want: "DuplicateSPIFFEID"},
		"clusterspiffeid-crd-missing":  {got: conditionReasonClusterSPIFFEIDCRDMissing, want: "ClusterSPIFFEIDCRDMissing"},
		"managed-output-apply-failed":  {got: conditionReasonManagedOutputApplyFailed, want: "ManagedOutputApplyFailed"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("reason = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func assertSuccessConditionSet(t *testing.T, binding *kleymv1alpha1.InferenceIdentityBinding) {
	t.Helper()

	assertConditionStatusOnBinding(t, binding, conditionTypeReady, metav1.ConditionTrue, conditionReasonReconciled)
	assertConditionStatusOnBinding(t, binding, conditionTypeInvalidRef, metav1.ConditionFalse, conditionReasonResolved)
	assertConditionStatusOnBinding(t, binding, conditionTypeUnsafeSelector, metav1.ConditionFalse, conditionReasonResolved)
	assertConditionStatusOnBinding(t, binding, conditionTypeConflict, metav1.ConditionFalse, conditionReasonResolved)
	assertConditionStatusOnBinding(t, binding, conditionTypeRenderFailure, metav1.ConditionFalse, conditionReasonResolved)
}

func assertPrimaryFailureCondition(
	t *testing.T,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	conditionType string,
	reason string,
) {
	t.Helper()

	ready := assertConditionStatusOnBinding(t, binding, conditionTypeReady, metav1.ConditionFalse, reason)
	if ready.Message == "" {
		t.Fatalf("Ready condition message is empty for reason %q", reason)
	}
	primary := assertConditionStatusOnBinding(t, binding, conditionType, metav1.ConditionTrue, reason)
	if primary.Message == "" {
		t.Fatalf("primary condition message is empty for reason %q", reason)
	}
	if ready.Message != primary.Message {
		t.Fatalf("Ready message = %q, want primary failure message %q", ready.Message, primary.Message)
	}

	for _, candidate := range []string{conditionTypeInvalidRef, conditionTypeUnsafeSelector, conditionTypeConflict, conditionTypeRenderFailure} {
		condition := meta.FindStatusCondition(binding.Status.Conditions, candidate)
		if condition == nil {
			t.Fatalf("missing condition %q", candidate)
		}
		if candidate == conditionType {
			continue
		}
		if condition.Status != metav1.ConditionFalse || condition.Reason != conditionReasonResolved {
			t.Fatalf("condition %q = %s/%s, want False/%s", candidate, condition.Status, condition.Reason, conditionReasonResolved)
		}
	}
}

func assertConditionStatusOnBinding(
	t *testing.T,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	conditionType string,
	expectedStatus metav1.ConditionStatus,
	expectedReason string,
) *metav1.Condition {
	t.Helper()

	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionType)
	if condition == nil {
		t.Fatalf("expected condition %q on %s/%s", conditionType, binding.Namespace, binding.Name)
	}
	if condition.Status != expectedStatus {
		t.Fatalf("condition %q status = %q, want %q", conditionType, condition.Status, expectedStatus)
	}
	if condition.Reason != expectedReason {
		t.Fatalf("condition %q reason = %q, want %q", conditionType, condition.Reason, expectedReason)
	}
	if condition.ObservedGeneration != binding.Generation {
		t.Fatalf("condition %q observedGeneration = %d, want %d", conditionType, condition.ObservedGeneration, binding.Generation)
	}
	return condition
}

func fetchBinding(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	name string,
) *kleymv1alpha1.InferenceIdentityBinding {
	t.Helper()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	key := types.NamespacedName{Namespace: testNamespace, Name: name}
	if err := k8sClient.Get(ctx, key, binding); err != nil {
		t.Fatalf("failed to fetch %s: %v", key, err)
	}
	return binding
}

func testPoolWithSelector(name string, selector map[string]any) *unstructured.Unstructured {
	pool := newTestPool()
	pool.SetName(name)
	pool.Object["spec"] = map[string]any{"selector": selector}
	return pool
}

type noMatchClient struct {
	client.Client
	getNoMatchGVK  schema.GroupVersionKind
	listNoMatchGVK schema.GroupVersionKind
}

func (c noMatchClient) Get(
	ctx context.Context,
	key types.NamespacedName,
	obj client.Object,
	opts ...client.GetOption,
) error {
	if c.getNoMatchGVK != (schema.GroupVersionKind{}) && obj.GetObjectKind().GroupVersionKind() == c.getNoMatchGVK {
		return &meta.NoKindMatchError{
			GroupKind:        c.getNoMatchGVK.GroupKind(),
			SearchedVersions: []string{c.getNoMatchGVK.Version},
		}
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c noMatchClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	if c.listNoMatchGVK != (schema.GroupVersionKind{}) && list.GetObjectKind().GroupVersionKind() == c.listNoMatchGVK {
		return &meta.NoKindMatchError{
			GroupKind:        c.listNoMatchGVK.GroupKind(),
			SearchedVersions: []string{c.listNoMatchGVK.Version},
		}
	}
	return c.Client.List(ctx, list, opts...)
}
