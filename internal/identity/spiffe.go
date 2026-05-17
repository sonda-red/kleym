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
package identity

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// DesiredClusterSPIFFEID renders the desired ClusterSPIFFEID object for a binding identity.
func DesiredClusterSPIFFEID(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity RenderedIdentity,
) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(ClusterSPIFFEIDGVK())
	object.SetName(identity.Name)
	object.SetLabels(ManagedClusterSPIFFEIDLabels(binding))

	selectorTemplates := make([]any, 0, len(identity.Selectors))
	for _, selector := range identity.Selectors {
		selectorTemplates = append(selectorTemplates, selector)
	}

	object.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          identity.SpiffeID,
		"podSelector":               identity.PodSelector,
		"workloadSelectorTemplates": selectorTemplates,
		"fallback":                  identity.Fallback,
		"hint":                      identity.Hint,
	}

	return object
}

// ManagedClusterSPIFFEIDLabels returns the labels used to find ClusterSPIFFEIDs owned by a binding.
func ManagedClusterSPIFFEIDLabels(binding *kleymv1alpha1.InferenceIdentityBinding) map[string]string {
	return map[string]string{
		ManagedByLabelKey:        ManagedByLabelValue,
		BindingNameLabelKey:      binding.Name,
		BindingNamespaceLabelKey: binding.Namespace,
	}
}
