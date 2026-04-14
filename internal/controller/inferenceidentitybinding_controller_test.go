/*
Copyright 2026 Kalin Daskalov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

var _ = Describe("InferenceIdentityBinding Controller", func() {
	cleanupBinding := func(ctx context.Context, key types.NamespacedName) {
		resource := &kleymv1alpha1.InferenceIdentityBinding{}
		err := k8sClient.Get(ctx, key, resource)
		if errors.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())

		controllerReconciler := &InferenceIdentityBindingReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			current := &kleymv1alpha1.InferenceIdentityBinding{}
			getErr := k8sClient.Get(ctx, key, current)
			g.Expect(errors.IsNotFound(getErr)).To(BeTrue())
		}, "5s", "100ms").Should(Succeed())
	}

	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		inferenceidentitybinding := &kleymv1alpha1.InferenceIdentityBinding{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind InferenceIdentityBinding")
			err := k8sClient.Get(ctx, typeNamespacedName, inferenceidentitybinding)
			if err != nil && errors.IsNotFound(err) {
				resource := &kleymv1alpha1.InferenceIdentityBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
						TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{
							Name: "example-target",
						},
						SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
						WorkloadSelectorTemplates: []string{
							"k8s:ns:default",
							"k8s:sa:inference-sa",
						},
						Mode: kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance InferenceIdentityBinding")
			cleanupBinding(ctx, typeNamespacedName)
		})

		It("should reconcile and surface unresolved references in status", func() {
			By("Reconciling the created resource")
			controllerReconciler := &InferenceIdentityBindingReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			By("updating status with invalid reference and adding finalizer")
			fetched := &kleymv1alpha1.InferenceIdentityBinding{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, fetched)).To(Succeed())
			Expect(fetched.Finalizers).To(ContainElement(inferenceIdentityBindingFinalizer))

			invalidRef := meta.FindStatusCondition(fetched.Status.Conditions, conditionTypeInvalidRef)
			Expect(invalidRef).NotTo(BeNil())
			Expect(invalidRef.Status).To(Equal(metav1.ConditionTrue))

			ready := meta.FindStatusCondition(fetched.Status.Conditions, conditionTypeReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		})

		It("should reconcile updates idempotently", func() {
			By("fetching the existing resource")
			resource := &kleymv1alpha1.InferenceIdentityBinding{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			By("updating the resource TargetRef.Name value")
			targetRefName := "bar"
			resource.Spec.TargetRef.Name = targetRefName
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			By("reconciling the updated resource")
			controllerReconciler := &InferenceIdentityBindingReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			By("reconciling again to verify idempotency")
			result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Context("When reconciling a non-existent resource", func() {
		ctx := context.Background()

		It("should not return an error for a missing resource", func() {
			By("Reconciling a resource that does not exist")
			controllerReconciler := &InferenceIdentityBindingReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-resource",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Context("When validating mode-specific API schema behavior", func() {
		ctx := context.Background()

		newResource := func(name string) *kleymv1alpha1.InferenceIdentityBinding {
			return &kleymv1alpha1.InferenceIdentityBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
					TargetRef: kleymv1alpha1.InferenceObjectiveTargetRef{
						Name: "example-target",
					},
					SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
					WorkloadSelectorTemplates: []string{
						"k8s:ns:default",
						"k8s:sa:inference-sa",
					},
				},
			}
		}

		It("should allow PerObjective mode with a containerDiscriminator", func() {
			resource := newResource("test-resource-perobjective")
			resource.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePerObjective
			resource.Spec.ContainerDiscriminator = &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "main",
			}

			By("creating a valid PerObjective resource")
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			DeferCleanup(func() {
				cleanupBinding(ctx, types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace})
			})

			By("reconciling the created resource")
			controllerReconciler := &InferenceIdentityBindingReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should default omitted mode to PerObjective and allow a containerDiscriminator", func() {
			resource := newResource("test-resource-default-perobjective")
			resource.Spec.ContainerDiscriminator = &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeImage,
				Value: "example/image:latest",
			}

			By("creating a resource without an explicit mode")
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			DeferCleanup(func() {
				cleanupBinding(ctx, types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace})
			})

			By("verifying the API server defaulted mode to PerObjective")
			fetched := &kleymv1alpha1.InferenceIdentityBinding{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace}, fetched)).To(Succeed())
			Expect(fetched.Spec.Mode).To(BeEquivalentTo(kleymv1alpha1.InferenceIdentityBindingModePerObjective))
			Expect(fetched.Spec.ContainerDiscriminator).NotTo(BeNil())
		})

		It("should reject containerDiscriminator when mode is PoolOnly", func() {
			resource := newResource("test-resource-poolonly-invalid")
			resource.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePoolOnly
			resource.Spec.ContainerDiscriminator = &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "main",
			}

			By("creating an invalid PoolOnly resource")
			err := k8sClient.Create(ctx, resource)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("containerDiscriminator must be empty when mode is PoolOnly"))
		})
	})

	Context("When setting up with a manager", func() {
		It("should register the controller with the manager successfully", func() {
			By("creating a manager")
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: k8sClient.Scheme(),
			})
			Expect(err).NotTo(HaveOccurred())

			By("setting up the controller with the manager")
			controllerReconciler := &InferenceIdentityBindingReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
			}
			err = controllerReconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
