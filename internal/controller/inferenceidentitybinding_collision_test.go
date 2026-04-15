package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const testNamespace = "default"

func TestReconcilePerObjectiveCollisionMarksAllAndBlocksClusterSPIFFEID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	objects := []client.Object{
		newTestPool(),
		newTestObjective("objective-a"),
		newTestObjective("objective-b"),
		newPerObjectiveBinding("binding-a", "objective-a"),
		newPerObjectiveBinding("binding-b", "objective-b"),
	}

	fakeRecorder := record.NewFakeRecorder(32)
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objects...).
			Build(),
		Scheme:   scheme,
		Recorder: fakeRecorder,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeReady, metav1.ConditionFalse, "IdentityCollision")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeReady, metav1.ConditionFalse, "IdentityCollision")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 0)
	assertEventContains(t, fakeRecorder.Events, "IdentityCollision")
}

func TestReconcilePerObjectiveCollisionResolutionClearsConflictAndResumes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	objects := []client.Object{
		newTestPool(),
		newTestObjective("objective-a"),
		newTestObjective("objective-b"),
		newPerObjectiveBinding("binding-a", "objective-a"),
		newPerObjectiveBinding("binding-b", "objective-b"),
	}

	fakeRecorder := record.NewFakeRecorder(64)
	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objects...).
			Build(),
		Scheme:   scheme,
		Recorder: fakeRecorder,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("initial Reconcile returned error: %v", err)
	}

	bindingB := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "binding-b"}, bindingB); err != nil {
		t.Fatalf("failed to get binding-b: %v", err)
	}
	bindingB.Spec.ContainerDiscriminator.Value = "sidecar"
	if err := reconciler.Update(ctx, bindingB); err != nil {
		t.Fatalf("failed to update binding-b: %v", err)
	}

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-b"},
	})
	if err != nil {
		t.Fatalf("reconcile binding-b returned error: %v", err)
	}
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("reconcile binding-a returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 2)
	assertEventContains(t, fakeRecorder.Events, "IdentityCollisionResolved")
}

func TestPoolOnlyBindingsAreNotSubjectToPerObjectiveCollisionRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	objects := []client.Object{
		newTestPool(),
		newTestObjective("objective-a"),
		newTestObjective("objective-b"),
		newPoolOnlyBinding("binding-a", "objective-a"),
		newPoolOnlyBinding("binding-b", "objective-b"),
	}

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objects...).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func TestChangingBindingToPoolOnlyResolvesPeerCollision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	objects := []client.Object{
		newTestPool(),
		newTestObjective("objective-a"),
		newTestObjective("objective-b"),
		newPerObjectiveBinding("binding-a", "objective-a"),
		newPerObjectiveBinding("binding-b", "objective-b"),
	}

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objects...).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("initial Reconcile returned error: %v", err)
	}
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision")

	bindingB := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "binding-b"}, bindingB); err != nil {
		t.Fatalf("failed to get binding-b: %v", err)
	}
	bindingB.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePoolOnly
	bindingB.Spec.ContainerDiscriminator = nil
	if err := reconciler.Update(ctx, bindingB); err != nil {
		t.Fatalf("failed to update binding-b: %v", err)
	}

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-b"},
	})
	if err != nil {
		t.Fatalf("reconcile binding-b returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
}

func TestPerObjectiveBindingsWithDifferentEffectiveSelectorsDoNotCollide(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)
	objects := []client.Object{
		newTestPool(),
		newTestObjective("objective-a"),
		newTestObjective("objective-b"),
		newPerObjectiveBindingWithServiceAccount("binding-a", "objective-a", "inference-sa-a"),
		newPerObjectiveBindingWithServiceAccount("binding-b", "objective-b", "inference-sa-b"),
	}

	reconciler := &InferenceIdentityBindingReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
			WithObjects(objects...).
			Build(),
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "binding-a"},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-b", conditionTypeConflict, metav1.ConditionFalse, "Resolved")
	assertConditionStatus(t, ctx, reconciler.Client, "binding-a", conditionTypeReady, metav1.ConditionTrue, "Reconciled")
	assertClusterSPIFFEIDCount(t, ctx, reconciler.Client, 1)
}

func newCollisionTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kleymv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add kleym scheme: %v", err)
	}
	registerUnstructuredGVK(scheme, clusterSPIFFEIDGVK)
	for _, gvk := range inferenceObjectiveGVKs {
		registerUnstructuredGVK(scheme, gvk)
	}
	for _, gvk := range inferencePoolGVKs {
		registerUnstructuredGVK(scheme, gvk)
	}

	return scheme
}

func registerUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}

func newTestPool() *unstructured.Unstructured {
	pool := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "model-server",
					},
				},
			},
		},
	}
	pool.SetGroupVersionKind(inferencePoolGVKs[0])
	pool.SetNamespace(testNamespace)
	pool.SetName("pool-a")
	return pool
}

func newTestObjective(name string) *unstructured.Unstructured {
	objective := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"poolRef": map[string]any{
					"name": "pool-a",
				},
			},
		},
	}
	objective.SetGroupVersionKind(inferenceObjectiveGVKs[0])
	objective.SetNamespace(testNamespace)
	objective.SetName(name)
	return objective
}

func newPerObjectiveBinding(name, objectiveName string) *kleymv1alpha1.InferenceIdentityBinding {
	return newPerObjectiveBindingWithServiceAccount(name, objectiveName, "inference-sa")
}

func newPerObjectiveBindingWithServiceAccount(
	name string,
	objectiveName string,
	serviceAccount string,
) *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{
				Name: objectiveName,
			},
			SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
			WorkloadSelectorTemplates: []string{
				"k8s:ns:" + testNamespace,
				"k8s:sa:" + serviceAccount,
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerDiscriminator: &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "main",
			},
		},
	}
}

func newPoolOnlyBinding(name, objectiveName string) *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{
				Name: objectiveName,
			},
			SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
			WorkloadSelectorTemplates: []string{
				"k8s:ns:" + testNamespace,
				"k8s:sa:inference-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		},
	}
}

func assertConditionStatus(
	t *testing.T,
	ctx context.Context,
	cli client.Client,
	name string,
	conditionType string,
	expectedStatus metav1.ConditionStatus,
	expectedReason string,
) {
	t.Helper()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, binding); err != nil {
		t.Fatalf("failed to fetch %s/%s: %v", testNamespace, name, err)
	}
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionType)
	if condition == nil {
		t.Fatalf("expected condition %q on %s/%s", conditionType, testNamespace, name)
	}
	if condition.Status != expectedStatus {
		t.Fatalf("condition %q on %s/%s status = %q, want %q", conditionType, testNamespace, name, condition.Status, expectedStatus)
	}
	if expectedReason != "" && condition.Reason != expectedReason {
		t.Fatalf("condition %q on %s/%s reason = %q, want %q", conditionType, testNamespace, name, condition.Reason, expectedReason)
	}
}

func assertClusterSPIFFEIDCount(t *testing.T, ctx context.Context, cli client.Client, expected int) {
	t.Helper()

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))
	if err := cli.List(ctx, list); err != nil {
		t.Fatalf("failed to list ClusterSPIFFEIDs: %v", err)
	}
	if len(list.Items) != expected {
		t.Fatalf("ClusterSPIFFEID count = %d, want %d", len(list.Items), expected)
	}
}

func assertEventContains(t *testing.T, events <-chan string, expectedSubstring string) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if strings.Contains(event, expectedSubstring) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event containing %q", expectedSubstring)
		}
	}
}
