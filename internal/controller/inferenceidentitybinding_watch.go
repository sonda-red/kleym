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

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// setupFieldIndexes registers controller-runtime field indexes on
// InferenceIdentityBinding objects. Field indexes allow efficient lookups
// (like a database index) without scanning every object in the namespace.
//
// Indexes registered:
//  1. objectiveRef.name — used by mapObjectiveToBindings to find bindings
//     whose optional objective subject changed.
//  2. poolRef.name — used by mapPoolToBindings to requeue bindings directly
//     anchored to a changed InferencePool.
//  3. effectiveMode — used by listCollisionCandidateBindings to find all
//     PerObjective bindings when peer names are unavailable.
//  4. containerName — used by listCollisionCandidateBindings to find bindings
//     with the same container boundary for collision detection.
func (r *InferenceIdentityBindingReconciler) setupFieldIndexes(mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(
		context.Background(),
		&kleymv1alpha1.InferenceIdentityBinding{},
		fieldIndexObjectiveRefName,
		func(rawObj client.Object) []string {
			return bindingObjectiveRefNameIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding objectiveRef.name: %w", err)
	}

	if err := indexer.IndexField(
		context.Background(),
		&kleymv1alpha1.InferenceIdentityBinding{},
		fieldIndexPoolRefName,
		func(rawObj client.Object) []string {
			return bindingPoolRefNameIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding poolRef.name: %w", err)
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
		fieldIndexContainerName,
		func(rawObj client.Object) []string {
			return bindingContainerNameIndexValue(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding containerName: %w", err)
	}

	return nil
}

func bindingObjectiveRefNameIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	if binding.Spec.ObjectiveRef == nil {
		return nil
	}

	objectiveName := strings.TrimSpace(binding.Spec.ObjectiveRef.Name)
	if objectiveName == "" {
		return nil
	}

	return []string{objectiveName}
}

func bindingPoolRefNameIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	poolName := strings.TrimSpace(binding.Spec.PoolRef.Name)
	if poolName == "" {
		return nil
	}

	return []string{poolName}
}

func bindingEffectiveModeIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	return []string{string(effectiveMode(binding.Spec.Mode))}
}

func bindingContainerNameIndexValue(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	containerName := binding.Spec.ContainerName
	if containerName == "" {
		return nil
	}

	return []string{containerName}
}

// reconcileWatchPredicate filters watch events to reduce unnecessary reconciliations.
// It allows creates and deletes unconditionally, but for updates it only triggers
// a reconcile when:
//   - spec.generation changed (meaning the spec was modified), or
//   - the deletion timestamp changed (object is being deleted).
//
// This prevents reconcile storms from status-only updates or metadata changes.
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

// mapPoolToBindings requeues bindings that directly anchor to a changed
// InferencePool. The field index keeps the fanout narrow, and the group filter
// avoids reconciling bindings pinned to the other supported GAIE pool group
// when both APIs serve pools with the same name.
func (r *InferenceIdentityBindingReconciler) mapPoolToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	bindings, err := r.listBindingsReferencingPool(ctx, object.GetNamespace(), object.GetName())
	if err != nil {
		return nil
	}

	poolGroup := object.GetObjectKind().GroupVersionKind().Group
	filtered := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindings))
	for i := range bindings {
		binding := bindings[i]
		refGroup := strings.TrimSpace(binding.Spec.PoolRef.Group)
		if refGroup != "" && poolGroup != "" && refGroup != poolGroup {
			continue
		}
		filtered = append(filtered, binding)
	}
	return requestsForBindings(filtered)
}

func (r *InferenceIdentityBindingReconciler) listBindingsReferencingObjective(
	ctx context.Context,
	namespace string,
	objectiveName string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	return r.listBindingsByField(ctx, namespace, fieldIndexObjectiveRefName, objectiveName)
}

func (r *InferenceIdentityBindingReconciler) listBindingsReferencingPool(
	ctx context.Context,
	namespace string,
	poolName string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	return r.listBindingsByField(ctx, namespace, fieldIndexPoolRefName, poolName)
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
