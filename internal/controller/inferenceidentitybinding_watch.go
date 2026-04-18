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
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func (r *InferenceIdentityBindingReconciler) setupFieldIndexes(mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(
		context.Background(),
		&kleymv1alpha1.InferenceIdentityBinding{},
		fieldIndexTargetRefName,
		func(rawObj client.Object) []string {
			return bindingTargetRefNameIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding targetRef.name: %w", err)
	}

	if err := indexer.IndexField(
		context.Background(),
		&kleymv1alpha1.InferenceIdentityBinding{},
		fieldIndexEffectiveMode,
		func(rawObj client.Object) []string {
			return bindingEffectiveModeIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding effective mode: %w", err)
	}

	if err := indexer.IndexField(
		context.Background(),
		&kleymv1alpha1.InferenceIdentityBinding{},
		fieldIndexContainerDiscriminatorKey,
		func(rawObj client.Object) []string {
			return bindingContainerDiscriminatorIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding container discriminator: %w", err)
	}

	return nil
}

func bindingTargetRefNameIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	targetName := strings.TrimSpace(binding.Spec.TargetRef.Name)
	if targetName == "" {
		return nil
	}

	return []string{targetName}
}

func bindingEffectiveModeIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	return []string{string(effectiveMode(binding.Spec.Mode))}
}

func bindingContainerDiscriminatorIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	key := containerDiscriminatorIndexKey(binding.Spec.ContainerDiscriminator)
	if key == "" {
		return nil
	}

	return []string{key}
}

func containerDiscriminatorIndexKey(discriminator *kleymv1alpha1.ContainerDiscriminator) string {
	if discriminator == nil {
		return ""
	}

	value := strings.TrimSpace(discriminator.Value)
	if value == "" {
		return ""
	}

	return fmt.Sprintf("%s|%s", discriminator.Type, value)
}

func reconcileWatchPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return true
			}
			if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
				return true
			}
			return deletionTimestampChanged(e.ObjectOld, e.ObjectNew)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return true
		},
	}
}

func deletionTimestampChanged(oldObject, newObject client.Object) bool {
	oldDeleting := oldObject.GetDeletionTimestamp() != nil
	newDeleting := newObject.GetDeletionTimestamp() != nil
	return oldDeleting != newDeleting
}

func (r *InferenceIdentityBindingReconciler) mapObjectiveToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	bindings, err := r.listBindingsReferencingObjective(ctx, object.GetNamespace(), object.GetName())
	if err != nil {
		return nil
	}

	return requestsForBindings(bindings)
}

func (r *InferenceIdentityBindingReconciler) mapPoolToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	namespace := object.GetNamespace()
	poolName := object.GetName()
	poolGroup := object.GetObjectKind().GroupVersionKind().Group
	objectiveNames := r.objectiveNamesReferencingPool(ctx, namespace, poolName, poolGroup)
	if len(objectiveNames) == 0 {
		return nil
	}

	requestsByKey := map[string]reconcile.Request{}
	objectiveNameList := make([]string, 0, len(objectiveNames))
	for objectiveName := range objectiveNames {
		objectiveNameList = append(objectiveNameList, objectiveName)
	}
	sort.Strings(objectiveNameList)

	for _, objectiveName := range objectiveNameList {
		bindings, err := r.listBindingsReferencingObjective(ctx, namespace, objectiveName)
		if err != nil {
			return nil
		}
		for i := range bindings {
			binding := bindings[i]
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: binding.Namespace,
					Name:      binding.Name,
				},
			}
			requestsByKey[request.String()] = request
		}
	}

	requestKeys := make([]string, 0, len(requestsByKey))
	for key := range requestsByKey {
		requestKeys = append(requestKeys, key)
	}
	sort.Strings(requestKeys)

	requests := make([]reconcile.Request, 0, len(requestKeys))
	for _, key := range requestKeys {
		request, exists := requestsByKey[key]
		if !exists {
			continue
		}
		requests = append(requests, request)
	}

	return requests
}

func (r *InferenceIdentityBindingReconciler) listBindingsReferencingObjective(
	ctx context.Context,
	namespace string,
	objectiveName string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	return r.listBindingsByField(ctx, namespace, fieldIndexTargetRefName, objectiveName)
}

func requestsForBindings(bindings []*kleymv1alpha1.InferenceIdentityBinding) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(bindings))
	for i := range bindings {
		binding := bindings[i]
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: binding.Namespace,
				Name:      binding.Name,
			},
		})
	}

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].String() < requests[j].String()
	})

	return requests
}

func (r *InferenceIdentityBindingReconciler) objectiveNamesReferencingPool(
	ctx context.Context,
	namespace string,
	poolName string,
	poolGroup string,
) map[string]struct{} {
	objectiveNames := map[string]struct{}{}

	for _, gvk := range inferenceObjectiveGVKs {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			continue
		}

		for i := range list.Items {
			objective := &list.Items[i]
			ref, err := extractPoolRef(objective, namespace)
			if err != nil {
				continue
			}
			if ref.Name != poolName {
				continue
			}
			if poolGroup != "" && ref.Group != "" && ref.Group != poolGroup {
				continue
			}
			objectiveNames[objective.GetName()] = struct{}{}
		}
	}

	return objectiveNames
}
