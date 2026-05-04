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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func (r *InferenceIdentityBindingReconciler) reconcileClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identities []renderedIdentity,
) error {
	logger := logf.FromContext(ctx)
	existing, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return err
	}
	logger.V(1).Info("listed managed ClusterSPIFFEIDs", "count", len(existing))

	existingByName := make(map[string]*unstructured.Unstructured, len(existing))
	for _, item := range existing {
		existingByName[item.GetName()] = item
	}

	desiredNames := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		desired := desiredClusterSPIFFEID(binding, identity)
		desiredNames[identity.Name] = struct{}{}

		current, exists := existingByName[identity.Name]
		if !exists {
			logger.Info(
				"creating managed ClusterSPIFFEID",
				logKeyClusterSPIFFEID, identity.Name,
				logKeyMode, identity.Mode,
				logKeySpiffeID, identity.SpiffeID,
			)
			if err := r.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			continue
		}

		if !clusterSPIFFEIDInSync(current, desired) {
			logger.Info(
				"updating drifted managed ClusterSPIFFEID",
				logKeyClusterSPIFFEID, identity.Name,
				logKeyMode, identity.Mode,
				logKeySpiffeID, identity.SpiffeID,
			)
			mergeDesiredClusterSPIFFEID(current, desired)
			if err := r.Update(ctx, current); err != nil {
				return err
			}
			continue
		}
		logger.V(1).Info(
			"managed ClusterSPIFFEID already in sync",
			logKeyClusterSPIFFEID, identity.Name,
			logKeyMode, identity.Mode,
			logKeySpiffeID, identity.SpiffeID,
		)
	}

	for name, object := range existingByName {
		if _, keep := desiredNames[name]; keep {
			continue
		}
		logger.Info("deleting stale managed ClusterSPIFFEID", logKeyClusterSPIFFEID, name)
		if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func desiredClusterSPIFFEID(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	object.SetName(identity.Name)
	object.SetLabels(map[string]string{
		managedByLabelKey:        managedByLabelValue,
		bindingNameLabelKey:      binding.Name,
		bindingNamespaceLabelKey: binding.Namespace,
	})

	selectorTemplates := make([]any, 0, len(identity.Selectors))
	for _, selector := range identity.Selectors {
		selectorTemplates = append(selectorTemplates, selector)
	}

	object.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          identity.SpiffeID,
		"podSelector":               identity.PodSelector,
		"workloadSelectorTemplates": selectorTemplates,
	}

	return object
}

func clusterSPIFFEIDInSync(current *unstructured.Unstructured, desired *unstructured.Unstructured) bool {
	currentSpec, _, currentErr := unstructured.NestedMap(current.Object, "spec")
	desiredSpec, _, desiredErr := unstructured.NestedMap(desired.Object, "spec")
	if currentErr != nil || desiredErr != nil {
		return false
	}

	if !reflect.DeepEqual(currentSpec, desiredSpec) {
		return false
	}

	currentLabels := current.GetLabels()
	for key, value := range desired.GetLabels() {
		if currentLabels[key] != value {
			return false
		}
	}

	return true
}

func mergeDesiredClusterSPIFFEID(current *unstructured.Unstructured, desired *unstructured.Unstructured) {
	currentSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	if current.Object == nil {
		current.Object = map[string]any{}
	}
	current.Object["spec"] = currentSpec

	labels := current.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range desired.GetLabels() {
		labels[key] = value
	}
	current.SetLabels(labels)
}

func (r *InferenceIdentityBindingReconciler) listManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]*unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))

	if err := r.List(
		ctx,
		list,
		client.MatchingLabels(map[string]string{
			managedByLabelKey:        managedByLabelValue,
			bindingNameLabelKey:      binding.Name,
			bindingNamespaceLabelKey: binding.Namespace,
		}),
	); err != nil {
		return nil, err
	}

	items := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, list.Items[i].DeepCopy())
	}

	return items, nil
}

func (r *InferenceIdentityBindingReconciler) cleanupManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	logger := logf.FromContext(ctx)
	objects, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		if meta.IsNoMatchError(err) {
			logger.Info("skipping managed ClusterSPIFFEID cleanup because CRD is unavailable")
			return nil
		}
		return err
	}
	if len(objects) == 0 {
		logger.V(1).Info("no managed ClusterSPIFFEIDs to clean up")
	}
	for _, object := range objects {
		logger.Info("deleting managed ClusterSPIFFEID during cleanup", logKeyClusterSPIFFEID, object.GetName())
		if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
