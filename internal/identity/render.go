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
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

// PlanIdentity computes desired identity state from resolved, read-only inputs.
func PlanIdentity(input PlanInput) (Plan, error) {
	binding := input.Binding
	mode := EffectiveMode(binding.Spec.Mode)
	if mode != kleymv1alpha1.InferenceIdentityBindingModePoolOnly &&
		mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"UnsupportedMode",
			fmt.Sprintf("unsupported mode %q", mode),
		)
	}
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective && strings.TrimSpace(input.ObjectiveName) == "" {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"MissingObjectiveRef",
			"objectiveRef is required when mode is PerObjective",
		)
	}
	if strings.TrimSpace(input.TrustDomain) == "" {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"MissingTrustDomain",
			"trustDomain must be configured before Kleym can render SPIFFE IDs",
		)
	}

	templateData := renderTemplateData{
		Namespace:     binding.Namespace,
		BindingName:   binding.Name,
		ObjectiveName: input.ObjectiveName,
		PoolName:      input.PoolName,
		Mode:          string(mode),
	}

	renderedSelectors, err := renderSafetySelectors(binding.Namespace, binding.Spec.ServiceAccountName)
	if err != nil {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"InvalidServiceAccountName",
			err.Error(),
		)
	}

	selectors := append(renderedSelectors, input.PoolDerivedSelectors...)
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		containerSelector, selectorErr := SelectorForContainerName(binding.Spec.ContainerName)
		if selectorErr != nil {
			return Plan{}, newStateError(
				ConditionTypeRenderFailure,
				"InvalidContainerName",
				selectorErr.Error(),
			)
		}
		selectors = append(selectors, containerSelector)
	} else if binding.Spec.ContainerName != "" {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"UnexpectedContainerName",
			"containerName must be empty when mode is PoolOnly",
		)
	}

	selectors = UniqueAndSorted(selectors)
	if err := validateRenderedSafetySelectors(binding.Namespace, selectors); err != nil {
		return Plan{}, newStateError(
			ConditionTypeUnsafeSelector,
			"UnsafeSelector",
			err.Error(),
		)
	}

	spiffeID := renderSPIFFEID(mode, input.TrustDomain, templateData)
	if !strings.HasPrefix(spiffeID, "spiffe://") {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			"InvalidSPIFFEID",
			fmt.Sprintf("computed SPIFFE ID %q must start with spiffe://", spiffeID),
		)
	}

	return Plan{
		Mode:         mode,
		SpiffeID:     spiffeID,
		Selectors:    selectors,
		PodSelector:  input.PodSelector,
		ObjectiveRef: input.ObjectiveName,
		PoolRef:      input.PoolName,
	}, nil
}

// renderSafetySelectors returns the mandatory namespace and service-account selectors.
func renderSafetySelectors(namespace, serviceAccountName string) ([]string, error) {
	if strings.TrimSpace(serviceAccountName) == "" {
		return nil, fmt.Errorf("serviceAccountName must not be empty")
	}
	if errs := validation.IsDNS1123Subdomain(serviceAccountName); len(errs) > 0 {
		return nil, fmt.Errorf("serviceAccountName %q is invalid: %s", serviceAccountName, strings.Join(errs, "; "))
	}
	return []string{
		"k8s:ns:" + namespace,
		"k8s:sa:" + serviceAccountName,
	}, nil
}

// SelectorForContainerName renders the SPIRE selector for the per-objective container boundary.
func SelectorForContainerName(containerName string) (string, error) {
	if strings.TrimSpace(containerName) == "" {
		return "", fmt.Errorf("containerName is required when mode is PerObjective")
	}
	if errs := validation.IsDNS1123Label(containerName); len(errs) > 0 {
		return "", fmt.Errorf("containerName %q is invalid: %s", containerName, strings.Join(errs, "; "))
	}
	return "k8s:container-name:" + containerName, nil
}

// renderSPIFFEID computes the fixed SPIFFE ID forms defined by docs/spec/operator.md.
func renderSPIFFEID(
	mode kleymv1alpha1.InferenceIdentityBindingMode,
	trustDomain string,
	data renderTemplateData,
) string {
	switch mode {
	case kleymv1alpha1.InferenceIdentityBindingModePoolOnly:
		return fmt.Sprintf("spiffe://%s/ns/%s/pool/%s", trustDomain, data.Namespace, data.PoolName)
	case kleymv1alpha1.InferenceIdentityBindingModePerObjective:
		return fmt.Sprintf("spiffe://%s/ns/%s/objective/%s", trustDomain, data.Namespace, data.ObjectiveName)
	default:
		return ""
	}
}

// validateRenderedSafetySelectors verifies that internally-rendered safety selectors are still present.
func validateRenderedSafetySelectors(namespace string, selectors []string) error {
	hasNamespaceSelector := false
	hasServiceAccountSelector := false

	for _, selector := range selectors {
		switch {
		case strings.HasPrefix(selector, "k8s:ns:"):
			hasNamespaceSelector = true
			ns := strings.TrimPrefix(selector, "k8s:ns:")
			if ns != namespace {
				return fmt.Errorf("selector %q escapes binding namespace %q", selector, namespace)
			}
		case strings.HasPrefix(selector, "k8s:sa:"):
			serviceAccount := strings.TrimPrefix(selector, "k8s:sa:")
			if strings.TrimSpace(serviceAccount) == "" {
				return fmt.Errorf("service account selector must not be empty")
			}
			hasServiceAccountSelector = true
		}
	}

	if !hasNamespaceSelector {
		return fmt.Errorf("selectors must include k8s:ns:%s", namespace)
	}
	if !hasServiceAccountSelector {
		return fmt.Errorf("selectors must include a k8s:sa:<service-account> selector")
	}

	return nil
}

// UniqueAndSorted canonicalizes selector lists for stable rendering and fingerprints.
func UniqueAndSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}

	unique := make([]string, 0, len(set))
	for value := range set {
		unique = append(unique, value)
	}
	sort.Strings(unique)

	return unique
}
