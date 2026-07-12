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
	"cmp"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// VariantLabelKey is the operator-owned Pod label key used for every identity boundary.
	VariantLabelKey = "identity.kleym.sonda.red/variant"
)

// ConflictCause identifies why two workload variants are not exclusive.
type ConflictCause string

const (
	// CauseVariantReuse reports equal variants that render distinct SPIFFE IDs.
	CauseVariantReuse ConflictCause = "VariantReuse"
	// CauseDuplicateSPIFFEID reports two bindings that render the same SPIFFE ID.
	CauseDuplicateSPIFFEID ConflictCause = "DuplicateSPIFFEID"
)

// VariantRecord is validated, resolved variant state used for exclusivity evaluation.
// BindingRef.Namespace is the variant namespace. SpiffeID must be a nonempty rendered SPIFFE ID.
// Pool selectors are intentionally absent because they do not prove boundary exclusivity.
type VariantRecord struct {
	BindingRef         types.NamespacedName
	ServiceAccountName string
	Variant            string
	SpiffeID           string
}

// VariantConflict describes one binding's conflict with a peer variant.
// The directional shape lets callers group records without coupling evaluation to API status types;
// peer fields contain only the data needed to project that binding's conflict status.
type VariantConflict struct {
	BindingRef     types.NamespacedName
	PeerBindingRef types.NamespacedName
	Cause          ConflictCause
	PeerSpiffeID   string
	PeerVariant    string
}

// ValidateVariant defensively enforces the admission contract before a variant
// can enter a SPIFFE ID or selector; see docs/spec/operator.md.
func ValidateVariant(variant string) error {
	if variant != strings.TrimSpace(variant) {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			"identityBoundary.variant must not include leading or trailing whitespace",
		)
	}
	if variant == "" {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			"identityBoundary.variant must not be empty",
		)
	}
	if errs := validation.IsValidLabelValue(variant); len(errs) > 0 {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			fmt.Sprintf("identityBoundary.variant %q is invalid: %s", variant, strings.Join(errs, "; ")),
		)
	}
	return nil
}

// EvaluateVariantConflicts returns both directional records for every non-exclusive pair.
// Callers must exclude invalid or unresolved bindings before evaluation. Every input record
// must have a nonempty rendered SPIFFE ID.
// Results are ordered by binding namespace and name, then by the peer namespace, peer name, cause,
// peer SPIFFE ID, and peer variant specified for status in docs/spec/operator.md.
func EvaluateVariantConflicts(variants []VariantRecord) []VariantConflict {
	var conflicts []VariantConflict
	for leftIndex := range variants {
		for rightIndex := leftIndex + 1; rightIndex < len(variants); rightIndex++ {
			left := variants[leftIndex]
			right := variants[rightIndex]
			cause, conflict := evaluateVariantPair(left, right)
			if !conflict {
				continue
			}

			conflicts = append(conflicts,
				variantConflict(left.BindingRef, right, cause),
				variantConflict(right.BindingRef, left, cause),
			)
		}
	}

	slices.SortFunc(conflicts, compareVariantConflicts)
	return conflicts
}

// variantConflict projects only the peer fields required by binding conflict status.
func variantConflict(bindingRef types.NamespacedName, peer VariantRecord, cause ConflictCause) VariantConflict {
	return VariantConflict{
		BindingRef:     bindingRef,
		PeerBindingRef: peer.BindingRef,
		Cause:          cause,
		PeerSpiffeID:   peer.SpiffeID,
		PeerVariant:    peer.Variant,
	}
}

// evaluateVariantPair applies only the structural exclusivity proofs from the operator spec.
func evaluateVariantPair(left, right VariantRecord) (ConflictCause, bool) {
	if left.SpiffeID == right.SpiffeID {
		return CauseDuplicateSPIFFEID, true
	}
	if left.BindingRef.Namespace != right.BindingRef.Namespace || left.ServiceAccountName != right.ServiceAccountName {
		return "", false
	}
	if left.Variant != right.Variant {
		return "", false
	}
	return CauseVariantReuse, true
}

// compareVariantConflicts provides a total order while keeping the documented status fields primary.
func compareVariantConflicts(left, right VariantConflict) int {
	return cmp.Or(
		cmp.Compare(left.BindingRef.Namespace, right.BindingRef.Namespace),
		cmp.Compare(left.BindingRef.Name, right.BindingRef.Name),
		cmp.Compare(left.PeerBindingRef.Namespace, right.PeerBindingRef.Namespace),
		cmp.Compare(left.PeerBindingRef.Name, right.PeerBindingRef.Name),
		cmp.Compare(left.Cause, right.Cause),
		cmp.Compare(left.PeerSpiffeID, right.PeerSpiffeID),
		cmp.Compare(left.PeerVariant, right.PeerVariant),
	)
}
