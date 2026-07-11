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
	"errors"
	"slices"
	"testing"
	"testing/quick"

	"k8s.io/apimachinery/pkg/types"
)

func TestValidateBoundary(t *testing.T) {
	t.Parallel()

	cases := map[string]Boundary{
		"missing":             {},
		"unreserved-key":      {LabelKey: "example.com/variant", LabelValue: "prefill"},
		"malformed-key":       {LabelKey: "identity.kleym.sonda.red/bad key", LabelValue: "prefill"},
		"empty-value":         {LabelKey: "identity.kleym.sonda.red/variant"},
		"malformed-value":     {LabelKey: "identity.kleym.sonda.red/variant", LabelValue: "bad/value"},
		"leading-whitespace":  {LabelKey: " identity.kleym.sonda.red/variant", LabelValue: "prefill"},
		"trailing-whitespace": {LabelKey: "identity.kleym.sonda.red/variant", LabelValue: "prefill "},
	}
	for name, boundary := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBoundary(boundary)
			var stateErr *StateError
			if !errors.As(err, &stateErr) {
				t.Fatalf("ValidateBoundary error = %v, want StateError", err)
			}
			if stateErr.ConditionType != ConditionTypeUnsafeSelector || stateErr.Reason != ReasonInvalidIdentityBoundary {
				t.Fatalf("condition/reason = %s/%s, want %s/%s", stateErr.ConditionType, stateErr.Reason, ConditionTypeUnsafeSelector, ReasonInvalidIdentityBoundary)
			}
		})
	}

	if err := ValidateBoundary(Boundary{
		LabelKey:   "identity.kleym.sonda.red/variant",
		LabelValue: "decode.v1",
	}); err != nil {
		t.Fatalf("ValidateBoundary valid input returned error: %v", err)
	}
}

func TestEvaluateBoundaryConflicts(t *testing.T) {
	t.Parallel()

	base := testBoundaryRecord("binding-a")
	cases := map[string]struct {
		change    func(*BoundaryRecord)
		wantCause ConflictCause
	}{
		"duplicate SPIFFE ID overrides different boundary shape": {
			change: func(peer *BoundaryRecord) {
				peer.BindingRef.Namespace = "other"
				peer.ServiceAccountName = "other-sa"
				peer.LabelKey = "identity.kleym.sonda.red/role"
				peer.LabelValue = "decode"
				peer.SpiffeID = base.SpiffeID
			},
			wantCause: CauseDuplicateSPIFFEID,
		},
		"namespace mismatch proves exclusivity": {
			change: func(peer *BoundaryRecord) {
				peer.BindingRef.Namespace = "other"
				peer.SpiffeID = testSpiffeID("other", peer.ServiceAccountName, "binding-b", peer.LabelValue)
			},
		},
		"service account mismatch proves exclusivity": {
			change: func(peer *BoundaryRecord) {
				peer.ServiceAccountName = "other-sa"
				peer.SpiffeID = testSpiffeID(peer.BindingRef.Namespace, "other-sa", "binding-b", peer.LabelValue)
			},
		},
		"same key and different value proves exclusivity": {
			change: func(peer *BoundaryRecord) {
				peer.LabelValue = "decode"
				peer.SpiffeID = testSpiffeID(peer.BindingRef.Namespace, peer.ServiceAccountName, "binding-b", peer.LabelValue)
			},
		},
		"same boundary with distinct SPIFFE IDs conflicts": {
			change:    func(_ *BoundaryRecord) {},
			wantCause: CauseBoundaryValueReuse,
		},
		"different keys with same value conflict": {
			change: func(peer *BoundaryRecord) {
				peer.LabelKey = "identity.kleym.sonda.red/role"
			},
			wantCause: CauseBoundaryKeyMismatch,
		},
		"different keys and values conflict": {
			change: func(peer *BoundaryRecord) {
				peer.LabelKey = "identity.kleym.sonda.red/role"
				peer.LabelValue = "decode"
				peer.SpiffeID = testSpiffeID(peer.BindingRef.Namespace, peer.ServiceAccountName, "binding-b", peer.LabelValue)
			},
			wantCause: CauseBoundaryKeyMismatch,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			peer := testBoundaryRecord("binding-b")
			tc.change(&peer)
			got := EvaluateBoundaryConflicts([]BoundaryRecord{base, peer})

			if tc.wantCause == "" {
				if len(got) != 0 {
					t.Fatalf("conflicts = %#v, want none", got)
				}
				return
			}
			want := []BoundaryConflict{
				boundaryConflict(base.BindingRef, peer, tc.wantCause),
				boundaryConflict(peer.BindingRef, base, tc.wantCause),
			}
			if !slices.Equal(got, want) {
				t.Fatalf("conflicts = %#v, want %#v", got, want)
			}
		})
	}
}

func TestEvaluateBoundaryConflictsIsSymmetricAndOrderIndependent(t *testing.T) {
	t.Parallel()

	property := func(namespaceIndex, serviceAccountIndex, keyIndex, valueIndex, spiffeIDIndex uint8) bool {
		namespaces := []string{"default", "tenant-a"}
		serviceAccounts := []string{"inference-sa", "decode-sa"}
		keys := []string{"identity.kleym.sonda.red/variant", "identity.kleym.sonda.red/role"}
		values := []string{"prefill", "decode"}
		pools := []string{"pool-a", "pool-b"}

		left := testBoundaryRecord("binding-a")
		left.BindingRef.Namespace = namespaces[int(namespaceIndex)%len(namespaces)]
		left.ServiceAccountName = serviceAccounts[int(serviceAccountIndex)%len(serviceAccounts)]
		left.LabelKey = keys[int(keyIndex)%len(keys)]
		left.LabelValue = values[int(valueIndex)%len(values)]
		left.SpiffeID = testSpiffeID(
			left.BindingRef.Namespace,
			left.ServiceAccountName,
			pools[int(spiffeIDIndex)%len(pools)],
			left.LabelValue,
		)
		right := testBoundaryRecord("binding-b")

		forward := EvaluateBoundaryConflicts([]BoundaryRecord{left, right})
		reverse := EvaluateBoundaryConflicts([]BoundaryRecord{right, left})
		return slices.Equal(forward, reverse)
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateBoundaryConflictsAlwaysClassifiesDuplicateSPIFFEIDs(t *testing.T) {
	t.Parallel()

	property := func(useOtherNamespace, useOtherServiceAccount, useOtherKey, useOtherValue bool) bool {
		left := testBoundaryRecord("binding-a")
		right := testBoundaryRecord("binding-b")
		if useOtherNamespace {
			right.BindingRef.Namespace = "tenant-a"
		}
		if useOtherServiceAccount {
			right.ServiceAccountName = "decode-sa"
		}
		if useOtherKey {
			right.LabelKey = "identity.kleym.sonda.red/role"
		}
		if useOtherValue {
			right.LabelValue = "decode"
		}
		right.SpiffeID = left.SpiffeID

		got := EvaluateBoundaryConflicts([]BoundaryRecord{left, right})
		return len(got) == 2 &&
			got[0].Cause == CauseDuplicateSPIFFEID &&
			got[1].Cause == CauseDuplicateSPIFFEID
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateBoundaryConflictsSortsMultiplePeers(t *testing.T) {
	t.Parallel()

	first := testBoundaryRecord("binding-a")
	second := testBoundaryRecord("binding-b")
	second.LabelKey = "identity.kleym.sonda.red/role"
	third := testBoundaryRecord("binding-c")

	inputs := [][]BoundaryRecord{
		{first, second, third},
		{first, third, second},
		{second, first, third},
		{second, third, first},
		{third, first, second},
		{third, second, first},
	}
	want := []BoundaryConflict{
		boundaryConflict(first.BindingRef, second, CauseBoundaryKeyMismatch),
		boundaryConflict(first.BindingRef, third, CauseBoundaryValueReuse),
		boundaryConflict(second.BindingRef, first, CauseBoundaryKeyMismatch),
		boundaryConflict(second.BindingRef, third, CauseBoundaryKeyMismatch),
		boundaryConflict(third.BindingRef, first, CauseBoundaryValueReuse),
		boundaryConflict(third.BindingRef, second, CauseBoundaryKeyMismatch),
	}
	for index, input := range inputs {
		if got := EvaluateBoundaryConflicts(input); !slices.Equal(got, want) {
			t.Fatalf("permutation %d conflicts = %#v, want %#v", index, got, want)
		}
	}
}

func TestEvaluateBoundaryConflictsMixedGraph(t *testing.T) {
	t.Parallel()

	a := testBoundaryRecord("binding-a")
	a.LabelKey = "identity.kleym.sonda.red/key-x"
	a.LabelValue = "one"
	b := testBoundaryRecord("binding-b")
	b.LabelKey = "identity.kleym.sonda.red/key-y"
	b.LabelValue = "one"
	c := testBoundaryRecord("binding-c")
	c.LabelKey = "identity.kleym.sonda.red/key-x"
	c.LabelValue = "two"

	want := []BoundaryConflict{
		{
			BindingRef:     a.BindingRef,
			PeerBindingRef: b.BindingRef,
			Cause:          CauseBoundaryKeyMismatch,
			PeerSpiffeID:   b.SpiffeID,
			PeerLabelKey:   b.LabelKey,
			PeerLabelValue: b.LabelValue,
		},
		{
			BindingRef:     b.BindingRef,
			PeerBindingRef: a.BindingRef,
			Cause:          CauseBoundaryKeyMismatch,
			PeerSpiffeID:   a.SpiffeID,
			PeerLabelKey:   a.LabelKey,
			PeerLabelValue: a.LabelValue,
		},
		{
			BindingRef:     b.BindingRef,
			PeerBindingRef: c.BindingRef,
			Cause:          CauseBoundaryKeyMismatch,
			PeerSpiffeID:   c.SpiffeID,
			PeerLabelKey:   c.LabelKey,
			PeerLabelValue: c.LabelValue,
		},
		{
			BindingRef:     c.BindingRef,
			PeerBindingRef: b.BindingRef,
			Cause:          CauseBoundaryKeyMismatch,
			PeerSpiffeID:   b.SpiffeID,
			PeerLabelKey:   b.LabelKey,
			PeerLabelValue: b.LabelValue,
		},
	}
	if got := EvaluateBoundaryConflicts([]BoundaryRecord{c, a, b}); !slices.Equal(got, want) {
		t.Fatalf("conflicts = %#v, want %#v", got, want)
	}
}

func testBoundaryRecord(name string) BoundaryRecord {
	return BoundaryRecord{
		BindingRef:         types.NamespacedName{Namespace: "default", Name: name},
		ServiceAccountName: "inference-sa",
		LabelKey:           "identity.kleym.sonda.red/variant",
		LabelValue:         "prefill",
		SpiffeID:           testSpiffeID("default", "inference-sa", name, "prefill"),
	}
}

func testSpiffeID(namespace, serviceAccount, pool, variant string) string {
	return "spiffe://example.org/ns/" + namespace + "/sa/" + serviceAccount + "/inference/pool/" + pool + "/variant/" + variant
}
