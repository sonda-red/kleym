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
package gaie

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	inferencePoolGVKs = []schema.GroupVersionKind{
		{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"},
	}
)

// InferencePoolGVKs returns the GAIE pool GVKs supported by kleym.
func InferencePoolGVKs() []schema.GroupVersionKind {
	return append([]schema.GroupVersionKind(nil), inferencePoolGVKs...)
}

// ResolvePoolGVKs falls back to all supported pool GVKs for tests and non-setup callers.
// Controller setup should pass discovered GVKs after narrowing against served resources.
func ResolvePoolGVKs(available []schema.GroupVersionKind) []schema.GroupVersionKind {
	if len(available) > 0 {
		return append([]schema.GroupVersionKind(nil), available...)
	}
	return InferencePoolGVKs()
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

// IsSupportedInferencePoolGroup checks poolRef groups against documented GAIE support.
func IsSupportedInferencePoolGroup(group string) bool {
	return len(supportedPoolGVKsForGroup(group)) > 0
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
