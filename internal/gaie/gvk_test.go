package gaie

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestInferencePoolGVKsReturnsCopy(t *testing.T) {
	t.Parallel()

	gvks := InferencePoolGVKs()
	if len(gvks) == 0 {
		t.Fatal("InferencePoolGVKs returned no GVKs")
	}

	gvks[0] = schema.GroupVersionKind{Group: "mutated.example.com", Version: "v1", Kind: "Mutated"}
	if got := InferencePoolGVKs()[0]; got == gvks[0] {
		t.Fatalf("InferencePoolGVKs exposed backing slice, first GVK = %v", got)
	}
}

func TestResolvePoolGVKsFallsBackToSupportedGVKs(t *testing.T) {
	t.Parallel()

	if got, want := ResolvePoolGVKs(nil), InferencePoolGVKs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvePoolGVKs(nil) = %v, want %v", got, want)
	}

	available := []schema.GroupVersionKind{{Group: "example.com", Version: "v1", Kind: "InferencePool"}}
	resolved := ResolvePoolGVKs(available)
	available[0].Group = "mutated.example.com"
	if resolved[0].Group != "example.com" {
		t.Fatalf("ResolvePoolGVKs exposed input slice, got %v", resolved)
	}
}

func TestCandidatePoolGVKsFilterByGroupAndFallbackToSupportedGroup(t *testing.T) {
	t.Parallel()

	poolGVKs := InferencePoolGVKs()
	poolCandidates := CandidatePoolGVKs(nil, "inference.networking.k8s.io")
	if len(poolCandidates) != 1 || poolCandidates[0].Group != "inference.networking.k8s.io" {
		t.Fatalf("CandidatePoolGVKs fallback = %v, want current GAIE group only", poolCandidates)
	}

	allPools := CandidatePoolGVKs(poolGVKs, "")
	poolGVKs[0].Group = "mutated.example.com"
	if allPools[0].Group != "inference.networking.k8s.io" {
		t.Fatalf("CandidatePoolGVKs exposed input slice, got %v", allPools)
	}

	legacyPools := CandidatePoolGVKs(nil, "inference.networking.x-k8s.io")
	if len(legacyPools) != 0 {
		t.Fatalf("CandidatePoolGVKs legacy fallback = %v, want no candidates", legacyPools)
	}
}

func TestSupportedInferencePoolGroups(t *testing.T) {
	t.Parallel()

	if !IsSupportedInferencePoolGroup("inference.networking.k8s.io") {
		t.Fatal("expected current GAIE InferencePool group to be supported")
	}
	if IsSupportedInferencePoolGroup("inference.networking.x-k8s.io") {
		t.Fatal("expected legacy x-k8s InferencePool group to be rejected")
	}
	if IsSupportedInferencePoolGroup("unsupported.example.com") {
		t.Fatal("expected unsupported InferencePool group to be rejected")
	}
}
