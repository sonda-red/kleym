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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

var _ = Describe("InferenceIdentityBinding Envtest Coverage", func() {
	ctx := context.Background()

	canonicalConditionTypes := []string{
		conditionTypeReady,
		conditionTypeInvalidRef,
		conditionTypeUnsafeSelector,
		conditionTypeRenderFailure,
		conditionTypeConflict,
	}

	newName := func(prefix string) string {
		return fmt.Sprintf("%s-%d", prefix, GinkgoRandomSeed()+time.Now().UnixNano()%100000)
	}

	cleanupBinding := func(key types.NamespacedName) {
		reconciler := &InferenceIdentityBindingReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

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

	createObjective := func(name, poolName string) *unstructured.Unstructured {
		objective := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"poolRef": map[string]any{"name": poolName},
			},
		}}
		objective.SetGroupVersionKind(inferenceObjectiveGVKs[0])
		objective.SetNamespace(testNamespace)
		objective.SetName(name)
		Expect(k8sClient.Create(ctx, objective)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, objective)
		})
		return objective
	}

	It("initializes the full canonical condition set for unsafe selectors", func() {
		poolName := newName("pool-unsafe")
		objectiveName := newName("objective-unsafe")
		bindingName := newName("binding-unsafe")

		createPool(poolName)
		createObjective(objectiveName, poolName)

		binding := &kleymv1alpha1.InferenceIdentityBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: bindingName},
			Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
				TargetRef:      kleymv1alpha1.InferenceObjectiveTargetRef{Name: objectiveName},
				SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
				WorkloadSelectorTemplates: []string{
					"k8s:sa:inference-sa",
				},
				Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() {
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingName})
		})

		reconciler := &InferenceIdentityBindingReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: bindingName}})
		Expect(err).NotTo(HaveOccurred())

		fetched := &kleymv1alpha1.InferenceIdentityBinding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingName}, fetched)).To(Succeed())

		for _, conditionType := range canonicalConditionTypes {
			condition := meta.FindStatusCondition(fetched.Status.Conditions, conditionType)
			Expect(condition).NotTo(BeNil(), "missing condition %s", conditionType)
			Expect(condition.ObservedGeneration).To(Equal(fetched.Generation), "unexpected observedGeneration for %s", conditionType)
		}

		unsafeSelector := meta.FindStatusCondition(fetched.Status.Conditions, conditionTypeUnsafeSelector)
		Expect(unsafeSelector).NotTo(BeNil())
		Expect(unsafeSelector.Status).To(Equal(metav1.ConditionTrue))
		Expect(unsafeSelector.Reason).To(Equal("UnsafeSelector"))
	})

	It("advances observedGeneration on all conditions after spec changes", func() {
		poolName := newName("pool-observed")
		objectiveName := newName("objective-observed")
		bindingName := newName("binding-observed")

		createPool(poolName)
		createObjective(objectiveName, poolName)

		binding := &kleymv1alpha1.InferenceIdentityBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: bindingName},
			Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
				TargetRef:      kleymv1alpha1.InferenceObjectiveTargetRef{Name: objectiveName},
				SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
				WorkloadSelectorTemplates: []string{
					"k8s:ns:default",
					"k8s:sa:inference-sa",
				},
				Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() {
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingName})
		})

		reconciler := &InferenceIdentityBindingReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: bindingName}})
		Expect(err).NotTo(HaveOccurred())

		fetched := &kleymv1alpha1.InferenceIdentityBinding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingName}, fetched)).To(Succeed())
		previousGeneration := fetched.Generation

		fetched.Spec.WorkloadSelectorTemplates = []string{
			"k8s:ns:default",
			"k8s:sa:inference-sa-v2",
		}
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

	It("propagates collision resolution to peers via manager reconciliation", func() {
		poolName := newName("pool-collision")
		objectiveAName := newName("objective-a")
		objectiveBName := newName("objective-b")
		bindingAName := newName("binding-a")
		bindingBName := newName("binding-b")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:                 k8sClient.Scheme(),
			Metrics:                server.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			Controller: config.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		reconciler := &InferenceIdentityBindingReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

		managerCtx, managerCancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		go func() {
			errCh <- mgr.Start(managerCtx)
		}()
		DeferCleanup(func() {
			managerCancel()
			Eventually(errCh, "5s", "100ms").Should(Receive(BeNil()))
		})

		createPool(poolName)
		createObjective(objectiveAName, poolName)
		createObjective(objectiveBName, poolName)

		bindingA := newPerObjectiveBinding(bindingAName, objectiveAName)
		bindingB := newPerObjectiveBinding(bindingBName, objectiveBName)
		Expect(k8sClient.Create(ctx, bindingA)).To(Succeed())
		Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())
		DeferCleanup(func() {
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingAName})
			cleanupBinding(types.NamespacedName{Namespace: testNamespace, Name: bindingBName})
		})

		Eventually(func(g Gomega) {
			currentA := &kleymv1alpha1.InferenceIdentityBinding{}
			currentB := &kleymv1alpha1.InferenceIdentityBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingAName}, currentA)).To(Succeed())
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingBName}, currentB)).To(Succeed())

			conflictA := meta.FindStatusCondition(currentA.Status.Conditions, conditionTypeConflict)
			conflictB := meta.FindStatusCondition(currentB.Status.Conditions, conditionTypeConflict)
			g.Expect(conflictA).NotTo(BeNil())
			g.Expect(conflictB).NotTo(BeNil())
			g.Expect(conflictA.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(conflictB.Status).To(Equal(metav1.ConditionTrue))
		}, "12s", "200ms").Should(Succeed())

		currentB := &kleymv1alpha1.InferenceIdentityBinding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingBName}, currentB)).To(Succeed())
		currentB.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePoolOnly
		currentB.Spec.ContainerDiscriminator = nil
		Expect(k8sClient.Update(ctx, currentB)).To(Succeed())

		Eventually(func(g Gomega) {
			currentA := &kleymv1alpha1.InferenceIdentityBinding{}
			updatedB := &kleymv1alpha1.InferenceIdentityBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingAName}, currentA)).To(Succeed())
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: bindingBName}, updatedB)).To(Succeed())

			conflictA := meta.FindStatusCondition(currentA.Status.Conditions, conditionTypeConflict)
			conflictB := meta.FindStatusCondition(updatedB.Status.Conditions, conditionTypeConflict)
			readyB := meta.FindStatusCondition(updatedB.Status.Conditions, conditionTypeReady)
			g.Expect(conflictA).NotTo(BeNil())
			g.Expect(conflictB).NotTo(BeNil())
			g.Expect(readyB).NotTo(BeNil())
			g.Expect(conflictA.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(conflictB.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(readyB.Status).To(Equal(metav1.ConditionTrue))
		}, "12s", "200ms").Should(Succeed())
	})
})
