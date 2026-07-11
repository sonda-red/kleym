package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

var _ = Describe("InferenceIdentityBinding Envtest Coverage", func() {
	ctx := context.Background()

	canonicalConditionTypes := []string{
		conditionTypeReady,
		conditionTypeInvalidRef,
		conditionTypeUnsafeSelector,
		conditionTypeConflict,
		conditionTypeRenderFailure,
	}

	newName := func(prefix string) string {
		return fmt.Sprintf("%s-%d", prefix, GinkgoRandomSeed()+time.Now().UnixNano()%100000)
	}

	cleanupBinding := func(key types.NamespacedName) {
		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient, Scheme: k8sClient.Scheme()}

		Eventually(func(g Gomega) {
			binding := &kleymv1alpha1.InferenceIdentityBinding{}
			err := k8sClient.Get(ctx, key, binding)
			if errors.IsNotFound(err) {
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			if binding.DeletionTimestamp.IsZero() {
				g.Expect(k8sClient.Delete(ctx, binding)).To(Succeed())
			}

			_, reconcileErr := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			g.Expect(reconcileErr).NotTo(HaveOccurred())
		}, "8s", "200ms").Should(Succeed())

		Eventually(func(g Gomega) {
			binding := &kleymv1alpha1.InferenceIdentityBinding{}
			g.Expect(errors.IsNotFound(k8sClient.Get(ctx, key, binding))).To(BeTrue())
		}, "8s", "200ms").Should(Succeed())
	}

	createPool := func(name string) *unstructured.Unstructured {
		pool := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{"app": "model-server"},
				},
			},
		}}
		pool.SetGroupVersionKind(inferencePoolGVKs[0])
		pool.SetNamespace(testNamespace)
		pool.SetName(name)
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, pool)
		})
		return pool
	}

	It("initializes the full canonical condition set for legacy invalid service accounts", func() {
		poolName := newName("pool-unsafe")
		bindingName := newName("binding-unsafe")

		createPool(poolName)

		binding := &kleymv1alpha1.InferenceIdentityBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  testNamespace,
				Name:       bindingName,
				Finalizers: []string{inferenceIdentityBindingFinalizer},
			},
			Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
				PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: poolName},
				ServiceAccountName: "inference-sa",
				IdentityBoundary:   testIdentityBoundary,
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() {
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingName})
		})

		legacyObjectClient := legacyInvalidServiceAccountClient{
			Client: k8sClient,
			key:    types.NamespacedName{Namespace: testNamespace, Name: bindingName},
		}
		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: legacyObjectClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: bindingName}})
		Expect(err).NotTo(HaveOccurred())

		fetched := &kleymv1alpha1.InferenceIdentityBinding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingName}, fetched)).To(Succeed())

		for _, conditionType := range canonicalConditionTypes {
			condition := meta.FindStatusCondition(fetched.Status.Conditions, conditionType)
			Expect(condition).NotTo(BeNil(), "missing condition %s", conditionType)
			Expect(condition.ObservedGeneration).To(Equal(fetched.Generation), "unexpected observedGeneration for %s", conditionType)
		}

		renderFailure := meta.FindStatusCondition(fetched.Status.Conditions, conditionTypeRenderFailure)
		Expect(renderFailure).NotTo(BeNil())
		Expect(renderFailure.Status).To(Equal(metav1.ConditionTrue))
		Expect(renderFailure.Reason).To(Equal("InvalidServiceAccountName"))
	})

	It("advances observedGeneration on all conditions after spec changes", func() {
		poolName := newName("pool-observed")
		bindingName := newName("binding-observed")

		createPool(poolName)

		binding := &kleymv1alpha1.InferenceIdentityBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: bindingName},
			Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
				PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: poolName},
				ServiceAccountName: "inference-sa",
				IdentityBoundary:   testIdentityBoundary,
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() {
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingName})
		})

		reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(), Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: bindingName}})
		Expect(err).NotTo(HaveOccurred())

		fetched := &kleymv1alpha1.InferenceIdentityBinding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingName}, fetched)).To(Succeed())
		previousGeneration := fetched.Generation

		fetched.Spec.ServiceAccountName = "inference-sa-v2"
		Expect(k8sClient.Update(ctx, fetched)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: bindingName}})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingName}, fetched)).To(Succeed())
		Expect(fetched.Generation).To(BeNumerically(">", previousGeneration))

		for _, conditionType := range canonicalConditionTypes {
			condition := meta.FindStatusCondition(fetched.Status.Conditions, conditionType)
			Expect(condition).NotTo(BeNil(), "missing condition %s", conditionType)
			Expect(condition.ObservedGeneration).To(Equal(fetched.Generation), "observedGeneration did not advance for %s", conditionType)
		}
	})

})

type legacyInvalidServiceAccountClient struct {
	client.Client
	key types.NamespacedName
}

// Get simulates an object persisted before service-account admission validation.
func (c legacyInvalidServiceAccountClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	options ...client.GetOption,
) error {
	if err := c.Client.Get(ctx, key, object, options...); err != nil {
		return err
	}
	if key != c.key {
		return nil
	}

	legacyBinding, ok := object.(*kleymv1alpha1.InferenceIdentityBinding)
	if ok {
		// The API server now rejects this value; runtime validation must handle legacy objects safely.
		legacyBinding.Spec.ServiceAccountName = "bad_service_account"
	}
	return nil
}
