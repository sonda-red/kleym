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
// The poolRef.name index lets mapPoolToBindings requeue only bindings anchored
// to a changed InferencePool. The managed-output name index lets
// mapClusterSPIFFEIDToBindings requeue only bindings that durably recorded the
// changed ClusterSPIFFEID as pending or owned output.
func (r *InferenceIdentityBindingReconciler) setupFieldIndexes(mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

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
		fieldIndexManagedClusterSPIFFEIDName,
		func(rawObj client.Object) []string {
			return bindingClusterSPIFFEIDNameIndexValues(rawObj)
		},
	); err != nil {
		return fmt.Errorf("failed to index InferenceIdentityBinding managed ClusterSPIFFEID name: %w", err)
	}

	return nil
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

// bindingClusterSPIFFEIDNameIndexValues projects the durable managed-output
// ownership fields used by the controller watch; labels are intentionally
// excluded because docs/spec/operator.md defines pending/owned status names as
// the ownership protocol.
func bindingClusterSPIFFEIDNameIndexValues(rawObj client.Object) []string {
	binding, ok := rawObj.(*kleymv1alpha1.InferenceIdentityBinding)
	if !ok {
		return nil
	}

	return compactIndexValues(
		binding.Status.PendingClusterSPIFFEIDName,
		binding.Status.OwnedClusterSPIFFEIDName,
	)
}

// compactIndexValues keeps field-index keys deterministic when legacy or
// partially patched status happens to contain duplicate or empty names.
func compactIndexValues(values ...string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
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

// mapPoolToBindings requeues bindings that directly anchor to a changed
// InferencePool. The field index keeps the fanout narrow, and the group filter
// avoids reconciling bindings pinned to a different GAIE pool group if support
// expands to multiple groups again.
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
	if len(filtered) == 0 {
		return nil
	}
	peers := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, peers, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}
	return requestsForBindingItems(peers.Items)
}

// mapBindingToPeers requeues namespace peers because any binding lifecycle or
// boundary change can change the structural exclusivity result for those peers.
func (r *InferenceIdentityBindingReconciler) mapBindingToPeers(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	peers := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, peers, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}
	return requestsForBindingItems(peers.Items)
}

// mapClusterSPIFFEIDToBindings requeues only bindings whose durable managed
// output status records the changed ClusterSPIFFEID name. Labels on the
// ClusterSPIFFEID are traceability metadata, not ownership proof.
func (r *InferenceIdentityBindingReconciler) mapClusterSPIFFEIDToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	bindings, err := r.listBindingsByManagedClusterSPIFFEIDName(ctx, object.GetName())
	if err != nil {
		return nil
	}
	return requestsForBindings(bindings)
}

func (r *InferenceIdentityBindingReconciler) listBindingsReferencingPool(
	ctx context.Context,
	namespace string,
	poolName string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	return r.listBindingsByField(ctx, namespace, fieldIndexPoolRefName, poolName)
}

// listBindingsByManagedClusterSPIFFEIDName performs the cluster-scoped reverse
// lookup needed for managed ClusterSPIFFEID watch events.
func (r *InferenceIdentityBindingReconciler) listBindingsByManagedClusterSPIFFEIDName(
	ctx context.Context,
	name string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	return r.listBindingsByField(ctx, "", fieldIndexManagedClusterSPIFFEIDName, name)
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

// requestsForBindingItems adapts listed API values to the shared sorted request helper.
func requestsForBindingItems(bindings []kleymv1alpha1.InferenceIdentityBinding) []reconcile.Request {
	pointers := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindings))
	for index := range bindings {
		pointers = append(pointers, bindings[index].DeepCopy())
	}
	return requestsForBindings(pointers)
}
