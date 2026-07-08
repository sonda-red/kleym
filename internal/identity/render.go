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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// PlanIdentity computes desired identity state from resolved, read-only inputs.
func PlanIdentity(input PlanInput) (Plan, error) {
	binding := input.Binding
	if strings.TrimSpace(input.TrustDomain) == "" {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			ReasonMissingTrustDomain,
			"trustDomain must be configured before Kleym can render SPIFFE IDs",
		)
	}

	templateData := renderTemplateData{
		Namespace:   binding.Namespace,
		BindingName: binding.Name,
		PoolName:    input.PoolName,
	}

	renderedSelectors, err := renderSafetySelectors(binding.Namespace, binding.Spec.ServiceAccountName)
	if err != nil {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			ReasonInvalidServiceAccountName,
			err.Error(),
		)
	}

	selectors := append(renderedSelectors, input.PoolDerivedSelectors...)
	selectors = UniqueAndSorted(selectors)
	if err := validateRenderedSafetySelectors(binding.Namespace, selectors); err != nil {
		return Plan{}, newStateError(
			ConditionTypeUnsafeSelector,
			ReasonUnsafeSelector,
			err.Error(),
		)
	}

	spiffeID := renderPoolSPIFFEID(input.TrustDomain, templateData)
	if !strings.HasPrefix(spiffeID, "spiffe://") {
		return Plan{}, newStateError(
			ConditionTypeRenderFailure,
			ReasonInvalidSPIFFEID,
			fmt.Sprintf("computed SPIFFE ID %q must start with spiffe://", spiffeID),
		)
	}

	return Plan{
		SpiffeID:    spiffeID,
		Selectors:   selectors,
		PodSelector: input.PodSelector,
		PoolRef:     input.PoolName,
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

// renderPoolSPIFFEID computes the fixed pool SPIFFE ID form defined by docs/spec/operator.md.
func renderPoolSPIFFEID(trustDomain string, data renderTemplateData) string {
	return fmt.Sprintf("spiffe://%s/ns/%s/pool/%s", trustDomain, data.Namespace, data.PoolName)
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

// SelectorFingerprint returns a stable sha256 fingerprint for the canonical selector set.
// The canonical selector contract is defined in docs/spec/operator.md.
func SelectorFingerprint(selectors []string) string {
	canonical := UniqueAndSorted(selectors)
	hash := sha256.New()
	for _, selector := range canonical {
		hash.Write([]byte(strconv.Itoa(len(selector))))
		hash.Write([]byte{0})
		hash.Write([]byte(selector))
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
