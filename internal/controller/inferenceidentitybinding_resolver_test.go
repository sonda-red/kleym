package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestReconcileSetsInvalidRefWhenPoolCannotBeResolved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	binding := newPoolOnlyBinding("binding-missing-pool", "")
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeInvalidRef, metav1.ConditionTrue, "TargetPoolNotFound")
	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionFalse, "TargetPoolNotFound")
}

func TestReconcilePoolOnlyDoesNotRequireObjective(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	binding := newPoolOnlyBinding("binding-pool-only", "")
	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, binding.Name, conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func TestReconcileUsesOperatorConfigForRenderedOutput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	binding := newPoolOnlyBinding("binding-operator-config", "")
	reconciler := &InferenceIdentityBindingReconciler{Config: OperatorConfig{
		TrustDomain:              "example.org",
		ClusterSPIFFEIDClassName: "kleym",
	},
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(newTestPool(), binding).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))
	if err := reconciler.List(ctx, list); err != nil {
		t.Fatalf("failed to list ClusterSPIFFEIDs: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("ClusterSPIFFEID count = %d, want 1", len(list.Items))
	}
	spiffeID, _, err := unstructured.NestedString(list.Items[0].Object, "spec", "spiffeIDTemplate")
	if err != nil {
		t.Fatalf("failed to read spec.spiffeIDTemplate: %v", err)
	}
	if spiffeID != "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill" {
		t.Fatalf("spiffeIDTemplate = %q, want variant-scoped pool target identity", spiffeID)
	}
	className, _, err := unstructured.NestedString(list.Items[0].Object, "spec", "className")
	if err != nil {
		t.Fatalf("failed to read spec.className: %v", err)
	}
	if className != "kleym" {
		t.Fatalf("className = %q, want kleym", className)
	}

	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: binding.Name}, current); err != nil {
		t.Fatalf("failed to read binding status: %v", err)
	}
	if current.Status.TrustDomain != "example.org" {
		t.Fatalf("status.trustDomain = %q, want example.org", current.Status.TrustDomain)
	}
	if current.Status.ClusterSPIFFEIDClassName != "kleym" {
		t.Fatalf("status.clusterSPIFFEIDClassName = %q, want kleym", current.Status.ClusterSPIFFEIDClassName)
	}
	if current.Status.RenderedClusterSPIFFEID == nil {
		t.Fatalf("status.renderedClusterSPIFFEID was not populated")
	}
	if current.Status.RenderedClusterSPIFFEID.Name != list.Items[0].GetName() {
		t.Fatalf(
			"status.renderedClusterSPIFFEID.name = %q, want %q",
			current.Status.RenderedClusterSPIFFEID.Name,
			list.Items[0].GetName(),
		)
	}
	if current.Status.RenderedClusterSPIFFEID.SpiffeID != spiffeID {
		t.Fatalf("status.renderedClusterSPIFFEID.spiffeID = %q, want %q", current.Status.RenderedClusterSPIFFEID.SpiffeID, spiffeID)
	}
	if current.Status.RenderedClusterSPIFFEID.SelectorFingerprint == "" {
		t.Fatalf("status.renderedClusterSPIFFEID.selectorFingerprint was not populated")
	}
}
