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

const reservedBoundaryLabelPrefix = "identity.kleym.sonda.red/"

// ConflictCause identifies why two identity boundaries are not exclusive.
type ConflictCause string

const (
	// CauseBoundaryValueReuse reports equal boundaries that render distinct SPIFFE IDs.
	CauseBoundaryValueReuse ConflictCause = "BoundaryValueReuse"
	// CauseBoundaryKeyMismatch reports different boundary keys in one namespace and service account.
	CauseBoundaryKeyMismatch ConflictCause = "BoundaryKeyMismatch"
	// CauseDuplicateSPIFFEID reports two bindings that render the same SPIFFE ID.
	CauseDuplicateSPIFFEID ConflictCause = "DuplicateSPIFFEID"
)

// BoundaryRecord is validated, resolved identity-boundary state used for exclusivity evaluation.
// BindingRef.Namespace is the boundary namespace. SpiffeID must be a nonempty rendered SPIFFE ID.
// Pool selectors are intentionally absent because they do not prove boundary exclusivity.
type BoundaryRecord struct {
	BindingRef         types.NamespacedName
	ServiceAccountName string
	LabelKey           string
	LabelValue         string
	SpiffeID           string
}

// BoundaryConflict describes one binding's conflict with a peer boundary.
// The directional shape lets callers group records without coupling evaluation to API status types;
// peer fields contain only the data needed to project that binding's conflict status.
type BoundaryConflict struct {
	BindingRef     types.NamespacedName
	PeerBindingRef types.NamespacedName
	Cause          ConflictCause
	PeerSpiffeID   string
	PeerLabelKey   string
	PeerLabelValue string
}

// ValidateBoundary defensively enforces the admission contract before a boundary
// can enter a SPIFFE ID or selector; see docs/spec/operator.md.
func ValidateBoundary(boundary Boundary) error {
	if boundary.LabelKey != strings.TrimSpace(boundary.LabelKey) ||
		boundary.LabelValue != strings.TrimSpace(boundary.LabelValue) {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			"identityBoundary label key and value must not include leading or trailing whitespace",
		)
	}
	if !strings.HasPrefix(boundary.LabelKey, reservedBoundaryLabelPrefix) {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			fmt.Sprintf("identityBoundary.labelKey %q must use reserved prefix %q", boundary.LabelKey, reservedBoundaryLabelPrefix),
		)
	}
	if errs := validation.IsQualifiedName(boundary.LabelKey); len(errs) > 0 {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			fmt.Sprintf("identityBoundary.labelKey %q is invalid: %s", boundary.LabelKey, strings.Join(errs, "; ")),
		)
	}
	if boundary.LabelValue == "" {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			"identityBoundary.labelValue must not be empty",
		)
	}
	if errs := validation.IsValidLabelValue(boundary.LabelValue); len(errs) > 0 {
		return newStateError(
			ConditionTypeUnsafeSelector,
			ReasonInvalidIdentityBoundary,
			fmt.Sprintf("identityBoundary.labelValue %q is invalid: %s", boundary.LabelValue, strings.Join(errs, "; ")),
		)
	}
	return nil
}

// EvaluateBoundaryConflicts returns both directional records for every non-exclusive pair.
// Callers must exclude invalid or unresolved bindings before evaluation. Every input record
// must have a nonempty rendered SPIFFE ID.
// Results are ordered by binding namespace and name, then by the peer namespace, peer name, cause,
// peer label key, and peer label value specified for status in docs/spec/operator.md. Peer SPIFFE ID
// provides a final deterministic tie-breaker.
func EvaluateBoundaryConflicts(boundaries []BoundaryRecord) []BoundaryConflict {
	var conflicts []BoundaryConflict
	for leftIndex := range boundaries {
		for rightIndex := leftIndex + 1; rightIndex < len(boundaries); rightIndex++ {
			left := boundaries[leftIndex]
			right := boundaries[rightIndex]
			cause, conflict := evaluateBoundaryPair(left, right)
			if !conflict {
				continue
			}

			conflicts = append(conflicts,
				boundaryConflict(left.BindingRef, right, cause),
				boundaryConflict(right.BindingRef, left, cause),
			)
		}
	}

	slices.SortFunc(conflicts, compareBoundaryConflicts)
	return conflicts
}

// boundaryConflict projects only the peer fields required by binding conflict status.
func boundaryConflict(bindingRef types.NamespacedName, peer BoundaryRecord, cause ConflictCause) BoundaryConflict {
	return BoundaryConflict{
		BindingRef:     bindingRef,
		PeerBindingRef: peer.BindingRef,
		Cause:          cause,
		PeerSpiffeID:   peer.SpiffeID,
		PeerLabelKey:   peer.LabelKey,
		PeerLabelValue: peer.LabelValue,
	}
}

// evaluateBoundaryPair applies only the structural exclusivity proofs from the operator spec.
func evaluateBoundaryPair(left, right BoundaryRecord) (ConflictCause, bool) {
	if left.SpiffeID == right.SpiffeID {
		return CauseDuplicateSPIFFEID, true
	}
	if left.BindingRef.Namespace != right.BindingRef.Namespace || left.ServiceAccountName != right.ServiceAccountName {
		return "", false
	}
	if left.LabelKey == right.LabelKey {
		if left.LabelValue != right.LabelValue {
			return "", false
		}
		return CauseBoundaryValueReuse, true
	}
	return CauseBoundaryKeyMismatch, true
}

// compareBoundaryConflicts provides a total order while keeping the documented status fields primary.
func compareBoundaryConflicts(left, right BoundaryConflict) int {
	return cmp.Or(
		cmp.Compare(left.BindingRef.Namespace, right.BindingRef.Namespace),
		cmp.Compare(left.BindingRef.Name, right.BindingRef.Name),
		cmp.Compare(left.PeerBindingRef.Namespace, right.PeerBindingRef.Namespace),
		cmp.Compare(left.PeerBindingRef.Name, right.PeerBindingRef.Name),
		cmp.Compare(left.Cause, right.Cause),
		cmp.Compare(left.PeerLabelKey, right.PeerLabelKey),
		cmp.Compare(left.PeerLabelValue, right.PeerLabelValue),
		cmp.Compare(left.PeerSpiffeID, right.PeerSpiffeID),
	)
}
