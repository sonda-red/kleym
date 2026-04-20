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
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	// maxDNSLabelLength is the maximum length of a single DNS label per RFC 1123.
	// ClusterSPIFFEID names must fit within this limit.
	maxDNSLabelLength = 63

	// nameHashBytes is the number of SHA1 hash bytes used in the ClusterSPIFFEID
	// name suffix. 4 bytes = 8 hex chars, which keeps names short while making
	// accidental collisions unlikely when the base name is truncated.
	nameHashBytes = 4
)

func (r *InferenceIdentityBindingReconciler) renderIdentity(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
) (renderedIdentity, error) {
	mode := effectiveMode(binding.Spec.Mode)
	if mode != kleymv1alpha1.InferenceIdentityBindingModePoolOnly &&
		mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"UnsupportedMode",
			fmt.Sprintf("unsupported mode %q", mode),
		)
	}

	podSelector, poolDerivedSelectors, err := deriveSelectorsFromPool(pool)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeUnsafeSelector,
			"InvalidPoolSelector",
			err.Error(),
		)
	}

	templateData := renderTemplateData{
		Namespace:     binding.Namespace,
		BindingName:   binding.Name,
		ObjectiveName: objective.GetName(),
		PoolName:      pool.GetName(),
		Mode:          string(mode),
	}
	if binding.Spec.ContainerDiscriminator != nil {
		templateData.ContainerDiscriminatorType = string(binding.Spec.ContainerDiscriminator.Type)
		templateData.ContainerDiscriminatorValue = binding.Spec.ContainerDiscriminator.Value
	}

	renderedSelectors, err := renderSelectorTemplates(binding.Spec.WorkloadSelectorTemplates, templateData)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"SelectorTemplateRenderFailed",
			err.Error(),
		)
	}

	selectors := append(renderedSelectors, poolDerivedSelectors...)
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		if binding.Spec.ContainerDiscriminator == nil {
			return renderedIdentity{}, newStateError(
				conditionTypeRenderFailure,
				"MissingContainerDiscriminator",
				"containerDiscriminator is required when mode is PerObjective",
			)
		}
		containerSelector, selectorErr := selectorForContainerDiscriminator(binding.Spec.ContainerDiscriminator)
		if selectorErr != nil {
			return renderedIdentity{}, newStateError(
				conditionTypeRenderFailure,
				"InvalidContainerDiscriminator",
				selectorErr.Error(),
			)
		}
		selectors = append(selectors, containerSelector)
	}

	selectors = uniqueAndSorted(selectors)
	if err := validateSafetySelectors(binding.Namespace, selectors); err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeUnsafeSelector,
			"UnsafeSelector",
			err.Error(),
		)
	}

	spiffeID, err := renderSPIFFEID(binding.Spec.SpiffeIDTemplate, mode, templateData)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"SPIFFEIDRenderFailed",
			err.Error(),
		)
	}
	if !strings.HasPrefix(spiffeID, "spiffe://") {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"InvalidSPIFFEID",
			fmt.Sprintf("computed SPIFFE ID %q must start with spiffe://", spiffeID),
		)
	}

	return renderedIdentity{
		Name:         buildClusterSPIFFEIDName(binding.Namespace, binding.Name, mode, spiffeID),
		Mode:         mode,
		SpiffeID:     spiffeID,
		Selectors:    selectors,
		PodSelector:  podSelector,
		ObjectiveRef: objective.GetName(),
		PoolRef:      pool.GetName(),
	}, nil
}

func (r *InferenceIdentityBindingReconciler) renderIdentityForBinding(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (renderedIdentity, error) {
	objective, err := r.resolveInferenceObjective(ctx, binding.Namespace, binding.Spec.TargetRef.Name)
	if err != nil {
		return renderedIdentity{}, err
	}

	poolRef, err := extractPoolRef(objective, binding.Namespace)
	if err != nil {
		return renderedIdentity{}, err
	}

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return renderedIdentity{}, err
	}

	return r.renderIdentity(binding, objective, pool)
}

// deriveSelectorsFromPool extracts pod-level selectors from an InferencePool.
//
// GAIE pools use spec.selector with matchLabels (map[string]string). When read
// through the unstructured API, the shape may vary by API version: some versions
// nest labels under matchLabels, others use a flat map. This function handles
// both: it checks for matchLabels first, then falls back to treating the entire
// selector map as flat labels if all values are strings.
//
// matchExpressions are detected and rejected — kleym only supports matchLabels
// for deterministic selector rendering.
//
// The returned selectors use the SPIRE Kubernetes Workload Attestor format:
// "k8s:pod-label:<key>:<value>".
//
// See docs/design/selector-safety.md for the safety model.
func deriveSelectorsFromPool(pool *unstructured.Unstructured) (map[string]any, []string, error) {
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
		valueText := strings.TrimSpace(fmt.Sprintf("%v", value))
		if key == "" || valueText == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty keys and values")
		}
		derivedSelectors = append(derivedSelectors, fmt.Sprintf("k8s:pod-label:%s:%s", key, valueText))
	}

	return selectorMap, derivedSelectors, nil
}

func renderSelectorTemplates(templates []string, data renderTemplateData) ([]string, error) {
	rendered := make([]string, 0, len(templates))
	for i, selectorTemplate := range templates {
		value, err := renderTemplate("selector", fmt.Sprintf("selector-%d", i), selectorTemplate, data)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, value)
	}
	return rendered, nil
}

func selectorForContainerDiscriminator(discriminator *kleymv1alpha1.ContainerDiscriminator) (string, error) {
	value := strings.TrimSpace(discriminator.Value)
	if value == "" {
		return "", fmt.Errorf("containerDiscriminator.value must not be empty")
	}

	switch discriminator.Type {
	case kleymv1alpha1.ContainerDiscriminatorTypeName:
		return "k8s:container-name:" + value, nil
	case kleymv1alpha1.ContainerDiscriminatorTypeImage:
		return "k8s:container-image:" + value, nil
	default:
		return "", fmt.Errorf("unsupported containerDiscriminator.type %q", discriminator.Type)
	}
}

// validateSafetySelectors enforces that the rendered selector set includes the
// two mandatory SPIRE Kubernetes Workload Attestor selectors:
//
//   - k8s:ns:<namespace>  — binds the identity to a specific namespace.
//     Without this, a ClusterSPIFFEID could match pods in other namespaces,
//     allowing identity escape across tenant boundaries.
//   - k8s:sa:<service-account> — binds the identity to a specific service account.
//     Without this, any pod in the namespace could receive the identity.
//
// These are SPIRE workload selector types documented at:
// https://github.com/spiffe/spire/blob/main/doc/plugin_agent_workloadattestor_k8s.md
func validateSafetySelectors(namespace string, selectors []string) error {
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

func renderSPIFFEID(
	customTemplate *string,
	mode kleymv1alpha1.InferenceIdentityBindingMode,
	data renderTemplateData,
) (string, error) {
	if customTemplate == nil {
		switch mode {
		case kleymv1alpha1.InferenceIdentityBindingModePoolOnly:
			return fmt.Sprintf("spiffe://%s/ns/%s/pool/%s", defaultTrustDomain, data.Namespace, data.PoolName), nil
		case kleymv1alpha1.InferenceIdentityBindingModePerObjective:
			return fmt.Sprintf("spiffe://%s/ns/%s/objective/%s", defaultTrustDomain, data.Namespace, data.ObjectiveName), nil
		default:
			return "", fmt.Errorf("unsupported mode %q", mode)
		}
	}

	return renderTemplate("spiffeID", "spiffeIDTemplate", *customTemplate, data)
}

func renderTemplate(kind, name, source string, data renderTemplateData) (string, error) {
	parsed, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("%s template parse failed: %w", kind, err)
	}

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("%s template render failed: %w", kind, err)
	}

	value := strings.TrimSpace(rendered.String())
	if value == "" {
		return "", fmt.Errorf("%s template rendered to an empty value", kind)
	}

	return value, nil
}

// buildClusterSPIFFEIDName produces a deterministic, DNS-label-safe name for a
// ClusterSPIFFEID resource.
//
// Format: <base>-<mode>-<hash>
//
//   - base: sanitized "kleym-<namespace>-<bindingName>", truncated to fit.
//   - mode: "pool" or "objective" — ensures PoolOnly and PerObjective names
//     never collide even for the same binding.
//   - hash: first nameHashBytes (4) bytes of SHA1(spiffeID), hex-encoded.
//     This suffix keeps names unique when the base is truncated due to the
//     maxDNSLabelLength (63 char) limit per RFC 1123.
func buildClusterSPIFFEIDName(
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

	// Reserve space for the mode text, hash suffix, and two hyphens.
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

// sanitizeDNSLabel converts an arbitrary string into a valid DNS label.
// DNS labels (RFC 952, RFC 1123) must contain only lowercase alphanumeric
// characters and hyphens, must not start or end with a hyphen, and must not
// contain consecutive hyphens. The lastHyphen flag below prevents runs of
// hyphens that would make the label invalid.
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

func uniqueAndSorted(values []string) []string {
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
