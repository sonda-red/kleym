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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// listBindingsByField looks up bindings using a controller-runtime field index.
// If the index is not available, the lookup falls back to listing all bindings
// in the namespace and filtering in memory.
func (r *InferenceIdentityBindingReconciler) listBindingsByField(
	ctx context.Context,
	namespace string,
	field string,
	value string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(
		ctx,
		bindingList,
		client.InNamespace(namespace),
		client.MatchingFields{field: value},
	); err != nil {
		if !isFieldLookupUnsupported(err) {
			return nil, err
		}
		return r.listBindingsByFieldFallback(ctx, namespace, field, value)
	}

	result := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		result = append(result, bindingList.Items[i].DeepCopy())
	}

	return result, nil
}

func (r *InferenceIdentityBindingReconciler) listBindingsByFieldFallback(
	ctx context.Context,
	namespace string,
	field string,
	value string,
) ([]*kleymv1alpha1.InferenceIdentityBinding, error) {
	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	result := make([]*kleymv1alpha1.InferenceIdentityBinding, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		binding := bindingList.Items[i].DeepCopy()
		if bindingMatchesField(binding, field, value) {
			result = append(result, binding)
		}
	}

	return result, nil
}

func bindingMatchesField(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	field string,
	value string,
) bool {
	switch field {
	case fieldIndexPoolRefName:
		return strings.TrimSpace(binding.Spec.PoolRef.Name) == value
	default:
		return false
	}
}

func isFieldLookupUnsupported(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "index with name") ||
		strings.Contains(errText, "field label not supported")
}
