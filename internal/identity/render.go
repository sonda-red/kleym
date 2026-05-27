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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	// maxDNSLabelLength is the maximum length of a single DNS label per RFC 1123.
	maxDNSLabelLength = 63

	// nameHashBytes controls the deterministic ClusterSPIFFEID name hash suffix length.
	nameHashBytes = 4
)

// RenderIdentity computes desired identity state without mutating Kubernetes resources.
func RenderIdentity(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
) (RenderedIdentity, error) {
	mode := EffectiveMode(binding.Spec.Mode)
	if mode != kleymv1alpha1.InferenceIdentityBindingModePoolOnly &&
		mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		return RenderedIdentity{}, newStateError(
			ConditionTypeRenderFailure,
			"UnsupportedMode",
			fmt.Sprintf("unsupported mode %q", mode),
		)
	}
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective && objective == nil {
		return RenderedIdentity{}, newStateError(
			ConditionTypeRenderFailure,
			"MissingObjectiveRef",
			"objectiveRef is required when mode is PerObjective",
		)
	}

	podSelector, poolDerivedSelectors, err := DeriveSelectorsFromPool(pool)
	if err != nil {
		return RenderedIdentity{}, newStateError(
			ConditionTypeUnsafeSelector,
			"InvalidPoolSelector",
			err.Error(),
		)
	}

	objectiveName := ""
	if objective != nil {
		objectiveName = objective.GetName()
	}

	templateData := renderTemplateData{
		Namespace:     binding.Namespace,
		BindingName:   binding.Name,
		ObjectiveName: objectiveName,
		PoolName:      pool.GetName(),
		Mode:          string(mode),
	}

	renderedSelectors, err := renderSafetySelectors(binding.Namespace, binding.Spec.ServiceAccountName)
	if err != nil {
		return RenderedIdentity{}, newStateError(
			ConditionTypeRenderFailure,
			"InvalidServiceAccountName",
			err.Error(),
		)
	}

	selectors := append(renderedSelectors, poolDerivedSelectors...)
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		containerSelector, selectorErr := SelectorForContainerName(binding.Spec.ContainerName)
		if selectorErr != nil {
			return RenderedIdentity{}, newStateError(
				ConditionTypeRenderFailure,
				"InvalidContainerName",
				selectorErr.Error(),
			)
		}
		selectors = append(selectors, containerSelector)
	} else if binding.Spec.ContainerName != "" {
		return RenderedIdentity{}, newStateError(
			ConditionTypeRenderFailure,
			"UnexpectedContainerName",
			"containerName must be empty when mode is PoolOnly",
		)
	}

	selectors = UniqueAndSorted(selectors)
	if err := ValidateSafetySelectors(binding.Namespace, selectors); err != nil {
		return RenderedIdentity{}, newStateError(
			ConditionTypeUnsafeSelector,
			"UnsafeSelector",
			err.Error(),
		)
	}

	spiffeID := renderSPIFFEID(mode, templateData)
	if !strings.HasPrefix(spiffeID, "spiffe://") {
		return RenderedIdentity{}, newStateError(
			ConditionTypeRenderFailure,
			"InvalidSPIFFEID",
			fmt.Sprintf("computed SPIFFE ID %q must start with spiffe://", spiffeID),
		)
	}

	return RenderedIdentity{
		Name:         BuildClusterSPIFFEIDName(binding.Namespace, binding.Name, mode, spiffeID),
		Mode:         mode,
		SpiffeID:     spiffeID,
		Selectors:    selectors,
		PodSelector:  podSelector,
		ObjectiveRef: objectiveName,
		PoolRef:      pool.GetName(),
		Hint:         BuildClusterSPIFFEIDHint(binding),
		Fallback:     false,
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
	data renderTemplateData,
) string {
	switch mode {
	case kleymv1alpha1.InferenceIdentityBindingModePoolOnly:
		return fmt.Sprintf("spiffe://%s/ns/%s/pool/%s", defaultTrustDomain, data.Namespace, data.PoolName)
	case kleymv1alpha1.InferenceIdentityBindingModePerObjective:
		return fmt.Sprintf("spiffe://%s/ns/%s/objective/%s", defaultTrustDomain, data.Namespace, data.ObjectiveName)
	default:
		return ""
	}
}

// BuildClusterSPIFFEIDHint builds the traceability hint for a generated ClusterSPIFFEID.
func BuildClusterSPIFFEIDHint(binding *kleymv1alpha1.InferenceIdentityBinding) string {
	return binding.Namespace + "/" + binding.Name
}

// DeriveSelectorsFromPool extracts deterministic pod-level selectors from an InferencePool.
func DeriveSelectorsFromPool(pool *unstructured.Unstructured) (map[string]any, []string, error) {
	selectorMap, found, err := unstructured.NestedMap(pool.Object, "spec", "selector")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode pool spec.selector: %w", err)
	}
	if !found || len(selectorMap) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector must be set")
	}

	var matchLabels map[string]any
	if rawMatchLabels, hasMatchLabels := selectorMap["matchLabels"]; hasMatchLabels {
		typedMatchLabels, ok := rawMatchLabels.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must be an object")
		}
		matchLabels = typedMatchLabels
	} else {
		isFlatSelector := true
		for _, value := range selectorMap {
			if _, ok := value.(string); !ok {
				isFlatSelector = false
				break
			}
		}
		if !isFlatSelector {
			return nil, nil, fmt.Errorf("pool selector must use matchLabels for deterministic rendering")
		}
		matchLabels = selectorMap
		selectorMap = map[string]any{"matchLabels": matchLabels}
	}

	if rawExpressions, hasExpressions := selectorMap["matchExpressions"]; hasExpressions {
		expressions, ok := rawExpressions.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions must be an array")
		}
		if len(expressions) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions are not supported")
		}
	}

	if len(matchLabels) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must not be empty")
	}

	derivedSelectors := make([]string, 0, len(matchLabels))
	for key, value := range matchLabels {
		valueText, ok := value.(string)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels[%q] must be a string", key)
		}
		if key == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty keys")
		}
		if valueText == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty values")
		}
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels key %q is invalid: %s", key, strings.Join(errs, "; "))
		}
		if errs := validation.IsValidLabelValue(valueText); len(errs) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels[%q] value %q is invalid: %s", key, valueText, strings.Join(errs, "; "))
		}
		derivedSelectors = append(derivedSelectors, fmt.Sprintf("k8s:pod-label:%s:%s", key, valueText))
	}

	return selectorMap, derivedSelectors, nil
}

// ValidateSafetySelectors enforces namespace and service account safety selectors.
func ValidateSafetySelectors(namespace string, selectors []string) error {
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

// BuildClusterSPIFFEIDName produces a deterministic, DNS-label-safe ClusterSPIFFEID name.
func BuildClusterSPIFFEIDName(
	namespace string,
	bindingName string,
	mode kleymv1alpha1.InferenceIdentityBindingMode,
	spiffeID string,
) string {
	modeText := "pool"
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		modeText = "objective"
	}

	hashSum := sha1.Sum([]byte(spiffeID))
	hashSuffix := hex.EncodeToString(hashSum[:nameHashBytes])
	base := sanitizeDNSLabel(fmt.Sprintf("%s-%s-%s", defaultNameValue, namespace, bindingName))

	maxBaseLen := maxDNSLabelLength - len(modeText) - len(hashSuffix) - 2
	if maxBaseLen < 1 {
		maxBaseLen = 1
	}
	if len(base) > maxBaseLen {
		base = strings.Trim(base[:maxBaseLen], "-")
		if base == "" {
			base = defaultNameValue
		}
	}

	return fmt.Sprintf("%s-%s-%s", base, modeText, hashSuffix)
}

func sanitizeDNSLabel(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return defaultNameValue
	}

	var labelBuilder strings.Builder
	lastHyphen := false
	for _, character := range lower {
		isAlphaNum := (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
		if isAlphaNum {
			labelBuilder.WriteRune(character)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			labelBuilder.WriteRune('-')
			lastHyphen = true
		}
	}

	sanitized := strings.Trim(labelBuilder.String(), "-")
	if sanitized == "" {
		return defaultNameValue
	}

	return sanitized
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
