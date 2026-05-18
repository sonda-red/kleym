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

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	inferenceObjectiveGVKs = []schema.GroupVersionKind{
		{Group: "inference.networking.x-k8s.io", Version: "v1alpha2", Kind: "InferenceObjective"},
		{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferenceObjective"},
	}
	inferencePoolGVKs = []schema.GroupVersionKind{
		{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"},
		{Group: "inference.networking.x-k8s.io", Version: "v1alpha2", Kind: "InferencePool"},
	}
	clusterSPIFFEIDGVK = schema.GroupVersionKind{
		Group:   "spire.spiffe.io",
		Version: "v1alpha1",
		Kind:    "ClusterSPIFFEID",
	}
)

// InferenceObjectiveGVKs returns the GAIE objective GVKs supported by kleym.
func InferenceObjectiveGVKs() []schema.GroupVersionKind {
	return append([]schema.GroupVersionKind(nil), inferenceObjectiveGVKs...)
}

// InferencePoolGVKs returns the GAIE pool GVKs supported by kleym.
func InferencePoolGVKs() []schema.GroupVersionKind {
	return append([]schema.GroupVersionKind(nil), inferencePoolGVKs...)
}

// ClusterSPIFFEIDGVK returns the SPIRE Controller Manager ClusterSPIFFEID GVK.
func ClusterSPIFFEIDGVK() schema.GroupVersionKind {
	return clusterSPIFFEIDGVK
}

// ResolveObjectiveGVKs falls back to all supported objective GVKs when discovery has not narrowed them.
func ResolveObjectiveGVKs(available []schema.GroupVersionKind) []schema.GroupVersionKind {
	if len(available) > 0 {
		return append([]schema.GroupVersionKind(nil), available...)
	}
	return InferenceObjectiveGVKs()
}

// ResolvePoolGVKs falls back to all supported pool GVKs when discovery has not narrowed them.
func ResolvePoolGVKs(available []schema.GroupVersionKind) []schema.GroupVersionKind {
	if len(available) > 0 {
		return append([]schema.GroupVersionKind(nil), available...)
	}
	return InferencePoolGVKs()
}

// CandidateObjectiveGVKs narrows objective lookup to a requested GAIE group.
func CandidateObjectiveGVKs(candidates []schema.GroupVersionKind, group string) []schema.GroupVersionKind {
	if group == "" {
		return append([]schema.GroupVersionKind(nil), candidates...)
	}

	filtered := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if gvk.Group == group {
			filtered = append(filtered, gvk)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}

	return supportedObjectiveGVKsForGroup(group)
}

// CandidatePoolGVKs narrows pool lookup to a requested GAIE group.
func CandidatePoolGVKs(candidates []schema.GroupVersionKind, group string) []schema.GroupVersionKind {
	if group == "" {
		return append([]schema.GroupVersionKind(nil), candidates...)
	}

	filtered := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if gvk.Group == group {
			filtered = append(filtered, gvk)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}

	return supportedPoolGVKsForGroup(group)
}

// IsSupportedInferenceObjectiveGroup checks objectiveRef groups against documented GAIE support.
func IsSupportedInferenceObjectiveGroup(group string) bool {
	return len(supportedObjectiveGVKsForGroup(group)) > 0
}

// IsSupportedInferencePoolGroup checks poolRef groups against documented GAIE support.
func IsSupportedInferencePoolGroup(group string) bool {
	return len(supportedPoolGVKsForGroup(group)) > 0
}

func supportedObjectiveGVKsForGroup(group string) []schema.GroupVersionKind {
	supported := make([]schema.GroupVersionKind, 0, len(inferenceObjectiveGVKs))
	for _, gvk := range inferenceObjectiveGVKs {
		if gvk.Group == group {
			supported = append(supported, gvk)
		}
	}
	return supported
}

func supportedPoolGVKsForGroup(group string) []schema.GroupVersionKind {
	supported := make([]schema.GroupVersionKind, 0, len(inferencePoolGVKs))
	for _, gvk := range inferencePoolGVKs {
		if gvk.Group == group {
			supported = append(supported, gvk)
		}
	}
	return supported
}
