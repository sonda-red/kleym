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
	"strings"
	"testing"
	"testing/quick"

	"k8s.io/apimachinery/pkg/types"
)

func TestValidateVariant(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing":             "",
		"malformed":           "bad/value",
		"leading-whitespace":  " prefill",
		"trailing-whitespace": "prefill ",
		"too-long":            strings.Repeat("a", 64),
	}
	for name, variant := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateVariant(variant)
			var stateErr *StateError
			if !errors.As(err, &stateErr) {
				t.Fatalf("ValidateVariant error = %v, want StateError", err)
			}
			if stateErr.ConditionType != ConditionTypeUnsafeSelector || stateErr.Reason != ReasonInvalidIdentityBoundary {
				t.Fatalf("condition/reason = %s/%s, want %s/%s", stateErr.ConditionType, stateErr.Reason, ConditionTypeUnsafeSelector, ReasonInvalidIdentityBoundary)
			}
		})
	}

	for _, variant := range []string{"a", "decode.v1", "variant_2"} {
		if err := ValidateVariant(variant); err != nil {
			t.Fatalf("ValidateVariant(%q) returned error: %v", variant, err)
		}
	}
}

func TestEvaluateVariantConflicts(t *testing.T) {
	t.Parallel()

	base := testVariantRecord("binding-a")
	cases := map[string]struct {
		change    func(*VariantRecord)
		wantCause ConflictCause
	}{
		"duplicate SPIFFE ID overrides a different boundary": {
			change: func(peer *VariantRecord) {
				peer.BindingRef.Namespace = "other"
				peer.ServiceAccountName = "other-sa"
				peer.Variant = "decode"
				peer.SpiffeID = base.SpiffeID
			},
			wantCause: CauseDuplicateSPIFFEID,
		},
		"namespace mismatch proves exclusivity": {
			change: func(peer *VariantRecord) {
				peer.BindingRef.Namespace = "other"
				peer.SpiffeID = testSpiffeID("other", peer.ServiceAccountName, "binding-b", peer.Variant)
			},
		},
		"service account mismatch proves exclusivity": {
			change: func(peer *VariantRecord) {
				peer.ServiceAccountName = "other-sa"
				peer.SpiffeID = testSpiffeID(peer.BindingRef.Namespace, "other-sa", "binding-b", peer.Variant)
			},
		},
		"different variant proves exclusivity": {
			change: func(peer *VariantRecord) {
				peer.Variant = "decode"
				peer.SpiffeID = testSpiffeID(peer.BindingRef.Namespace, peer.ServiceAccountName, "binding-b", peer.Variant)
			},
		},
		"variant reuse with distinct SPIFFE IDs conflicts": {
			change:    func(_ *VariantRecord) {},
			wantCause: CauseVariantReuse,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			peer := testVariantRecord("binding-b")
			tc.change(&peer)
			got := EvaluateVariantConflicts([]VariantRecord{base, peer})

			if tc.wantCause == "" {
				if len(got) != 0 {
					t.Fatalf("conflicts = %#v, want none", got)
				}
				return
			}
			want := []VariantConflict{
				variantConflict(base.BindingRef, peer, tc.wantCause),
				variantConflict(peer.BindingRef, base, tc.wantCause),
			}
			if !slices.Equal(got, want) {
				t.Fatalf("conflicts = %#v, want %#v", got, want)
			}
		})
	}
}

func TestEvaluateVariantConflictsIsSymmetricAndOrderIndependent(t *testing.T) {
	t.Parallel()

	property := func(namespaceIndex, serviceAccountIndex, variantIndex, spiffeIDIndex uint8) bool {
		namespaces := []string{"default", "tenant-a"}
		serviceAccounts := []string{"inference-sa", "decode-sa"}
		variants := []string{"prefill", "decode"}
		pools := []string{"pool-a", "pool-b"}

		left := testVariantRecord("binding-a")
		left.BindingRef.Namespace = namespaces[int(namespaceIndex)%len(namespaces)]
		left.ServiceAccountName = serviceAccounts[int(serviceAccountIndex)%len(serviceAccounts)]
		left.Variant = variants[int(variantIndex)%len(variants)]
		left.SpiffeID = testSpiffeID(
			left.BindingRef.Namespace,
			left.ServiceAccountName,
			pools[int(spiffeIDIndex)%len(pools)],
			left.Variant,
		)
		right := testVariantRecord("binding-b")

		forward := EvaluateVariantConflicts([]VariantRecord{left, right})
		reverse := EvaluateVariantConflicts([]VariantRecord{right, left})
		return slices.Equal(forward, reverse)
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateVariantConflictsAlwaysClassifiesDuplicateSPIFFEIDs(t *testing.T) {
	t.Parallel()

	property := func(useOtherNamespace, useOtherServiceAccount, useOtherVariant bool) bool {
		left := testVariantRecord("binding-a")
		right := testVariantRecord("binding-b")
		if useOtherNamespace {
			right.BindingRef.Namespace = "tenant-a"
		}
		if useOtherServiceAccount {
			right.ServiceAccountName = "decode-sa"
		}
		if useOtherVariant {
			right.Variant = "decode"
		}
		right.SpiffeID = left.SpiffeID

		got := EvaluateVariantConflicts([]VariantRecord{left, right})
		return len(got) == 2 &&
			got[0].Cause == CauseDuplicateSPIFFEID &&
			got[1].Cause == CauseDuplicateSPIFFEID
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateVariantConflictsSortsMultiplePeers(t *testing.T) {
	t.Parallel()

	first := testVariantRecord("binding-a")
	second := testVariantRecord("binding-b")
	third := testVariantRecord("binding-c")

	inputs := [][]VariantRecord{
		{first, second, third},
		{first, third, second},
		{second, first, third},
		{second, third, first},
		{third, first, second},
		{third, second, first},
	}
	want := []VariantConflict{
		variantConflict(first.BindingRef, second, CauseVariantReuse),
		variantConflict(first.BindingRef, third, CauseVariantReuse),
		variantConflict(second.BindingRef, first, CauseVariantReuse),
		variantConflict(second.BindingRef, third, CauseVariantReuse),
		variantConflict(third.BindingRef, first, CauseVariantReuse),
		variantConflict(third.BindingRef, second, CauseVariantReuse),
	}
	for index, input := range inputs {
		if got := EvaluateVariantConflicts(input); !slices.Equal(got, want) {
			t.Fatalf("permutation %d conflicts = %#v, want %#v", index, got, want)
		}
	}
}

func testVariantRecord(name string) VariantRecord {
	return VariantRecord{
		BindingRef:         types.NamespacedName{Namespace: "default", Name: name},
		ServiceAccountName: "inference-sa",
		Variant:            "prefill",
		SpiffeID:           testSpiffeID("default", "inference-sa", name, "prefill"),
	}
}

func testSpiffeID(namespace, serviceAccount, pool, variant string) string {
	return "spiffe://example.org/ns/" + namespace + "/sa/" + serviceAccount + "/inference/pool/" + pool + "/variant/" + variant
}
