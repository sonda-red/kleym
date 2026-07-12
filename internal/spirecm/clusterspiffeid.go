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
package spirecm

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

const (
	defaultNameValue = "kleym"

	// ManagedByLabelKey identifies ClusterSPIFFEID resources owned by kleym.
	ManagedByLabelKey = "kleym.sonda.red/managed-by"
	// ManagedByLabelValue is the stable managed-by label value for kleym resources.
	ManagedByLabelValue = defaultNameValue
	// BindingNameLabelKey records the source InferenceIdentityBinding name.
	BindingNameLabelKey = "kleym.sonda.red/binding-name"
	// BindingNamespaceLabelKey records the source InferenceIdentityBinding namespace.
	BindingNamespaceLabelKey = "kleym.sonda.red/binding-namespace"
	// OwnershipClaimIDAnnotationKey records the pending-create claim that produced an object.
	OwnershipClaimIDAnnotationKey = "kleym.sonda.red/ownership-claim-id"

	// maxDNSLabelLength is the maximum length of a single DNS label per RFC 1123.
	maxDNSLabelLength = 63

	// nameHashBytes controls the deterministic ClusterSPIFFEID name hash suffix length.
	nameHashBytes = 4
)

var clusterSPIFFEIDGVK = schema.GroupVersionKind{
	Group:   "spire.spiffe.io",
	Version: "v1alpha1",
	Kind:    "ClusterSPIFFEID",
}

// SetClusterSPIFFEIDOwnershipClaim records immutable create provenance on a
// newly rendered ClusterSPIFFEID. Reconciliation intentionally does not merge
// this annotation onto existing objects.
func SetClusterSPIFFEIDOwnershipClaim(object *unstructured.Unstructured, claimID string) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[OwnershipClaimIDAnnotationKey] = claimID
	object.SetAnnotations(annotations)
}

// ClusterSPIFFEIDOwnershipClaim returns the create claim carried by an object.
func ClusterSPIFFEIDOwnershipClaim(object *unstructured.Unstructured) string {
	return object.GetAnnotations()[OwnershipClaimIDAnnotationKey]
}

// ClusterSPIFFEIDGVK returns the SPIRE Controller Manager ClusterSPIFFEID GVK.
func ClusterSPIFFEIDGVK() schema.GroupVersionKind {
	return clusterSPIFFEIDGVK
}

// DesiredClusterSPIFFEID renders the desired ClusterSPIFFEID object for a binding identity plan.
func DesiredClusterSPIFFEID(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	plan identity.Plan,
	className string,
) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(ClusterSPIFFEIDGVK())
	object.SetName(BuildClusterSPIFFEIDName(
		binding.Namespace,
		binding.Name,
		plan.IdentityAnchor.Kind,
		plan.SpiffeID,
	))
	object.SetLabels(ManagedClusterSPIFFEIDLabels(binding))

	selectorTemplates := make([]any, 0, len(plan.Selectors))
	for _, selector := range plan.Selectors {
		selectorTemplates = append(selectorTemplates, selector)
	}

	spec := map[string]any{
		"spiffeIDTemplate":          plan.SpiffeID,
		"podSelector":               plan.PodSelector,
		"workloadSelectorTemplates": selectorTemplates,
		"fallback":                  RenderFallback(),
		"hint":                      BuildClusterSPIFFEIDHint(binding),
	}
	if className != "" {
		spec["className"] = className
	}
	object.Object["spec"] = spec

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

// BuildClusterSPIFFEIDName produces a deterministic, DNS-label-safe ClusterSPIFFEID name.
func BuildClusterSPIFFEIDName(
	namespace string,
	bindingName string,
	identityAnchorKind string,
	spiffeID string,
) string {
	modeText := sanitizeDNSLabel(identityAnchorKind)
	if modeText == "" {
		modeText = "identity"
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

// BuildClusterSPIFFEIDHint builds the traceability hint for a generated ClusterSPIFFEID.
func BuildClusterSPIFFEIDHint(binding *kleymv1alpha1.InferenceIdentityBinding) string {
	return binding.Namespace + "/" + binding.Name
}

// RenderFallback returns the managed ClusterSPIFFEID fallback value.
func RenderFallback() bool {
	return false
}

// ClusterSPIFFEIDInSync reports whether the current managed object matches desired fields.
func ClusterSPIFFEIDInSync(current *unstructured.Unstructured, desired *unstructured.Unstructured) bool {
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

// MergeDesiredClusterSPIFFEID copies desired managed fields onto the current object.
func MergeDesiredClusterSPIFFEID(current *unstructured.Unstructured, desired *unstructured.Unstructured) {
	desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	if current.Object == nil {
		current.Object = map[string]any{}
	}
	current.Object["spec"] = desiredSpec

	labels := current.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range desired.GetLabels() {
		labels[key] = value
	}
	current.SetLabels(labels)
}

func sanitizeDNSLabel(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
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
		return ""
	}

	return sanitized
}
