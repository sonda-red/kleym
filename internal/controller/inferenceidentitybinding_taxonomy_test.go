package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	reconciler := &InferenceIdentityBindingReconciler{
		Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
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
					Client:         base,
					listNoMatchGVK: clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"),
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
				Scheme: scheme,
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
		"unsafe-selector":              {got: identity.ReasonUnsafeSelector, want: "UnsafeSelector"},
		"missing-trust-domain":         {got: identity.ReasonMissingTrustDomain, want: "MissingTrustDomain"},
		"invalid-service-account-name": {got: identity.ReasonInvalidServiceAccountName, want: "InvalidServiceAccountName"},
		"invalid-spiffe-id":            {got: identity.ReasonInvalidSPIFFEID, want: "InvalidSPIFFEID"},
		"clusterspiffeid-crd-missing":  {got: conditionReasonClusterSPIFFEIDCRDMissing, want: "ClusterSPIFFEIDCRDMissing"},
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

	for _, candidate := range []string{conditionTypeInvalidRef, conditionTypeUnsafeSelector, conditionTypeRenderFailure} {
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
