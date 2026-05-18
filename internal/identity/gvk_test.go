package identity

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestInferenceObjectiveGVKsReturnsCopy(t *testing.T) {
	t.Parallel()

	gvks := InferenceObjectiveGVKs()
	if len(gvks) == 0 {
		t.Fatal("InferenceObjectiveGVKs returned no GVKs")
	}

	gvks[0] = schema.GroupVersionKind{Group: "mutated.example.com", Version: "v1", Kind: "Mutated"}
	if got := InferenceObjectiveGVKs()[0]; got == gvks[0] {
		t.Fatalf("InferenceObjectiveGVKs exposed backing slice, first GVK = %v", got)
	}
}

func TestResolveObjectiveGVKsFallsBackToSupportedGVKs(t *testing.T) {
	t.Parallel()

	if got, want := ResolveObjectiveGVKs(nil), InferenceObjectiveGVKs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveObjectiveGVKs(nil) = %v, want %v", got, want)
	}

	available := []schema.GroupVersionKind{{Group: "example.com", Version: "v1", Kind: "InferenceObjective"}}
	resolved := ResolveObjectiveGVKs(available)
	available[0].Group = "mutated.example.com"
	if resolved[0].Group != "example.com" {
		t.Fatalf("ResolveObjectiveGVKs exposed input slice, got %v", resolved)
	}
}

func TestCandidateGVKsFilterByGroupAndFallbackToSupportedGroup(t *testing.T) {
	t.Parallel()

	objectiveGVKs := InferenceObjectiveGVKs()
	poolGVKs := InferencePoolGVKs()

	objectiveCandidates := CandidateObjectiveGVKs(objectiveGVKs, "inference.networking.k8s.io")
	if len(objectiveCandidates) != 1 || objectiveCandidates[0].Group != "inference.networking.k8s.io" {
		t.Fatalf("CandidateObjectiveGVKs filtered = %v, want k8s group only", objectiveCandidates)
	}

	poolCandidates := CandidatePoolGVKs(nil, "inference.networking.x-k8s.io")
	if len(poolCandidates) != 1 || poolCandidates[0].Group != "inference.networking.x-k8s.io" {
		t.Fatalf("CandidatePoolGVKs fallback = %v, want x-k8s group only", poolCandidates)
	}

	allPools := CandidatePoolGVKs(poolGVKs, "")
	poolGVKs[0].Group = "mutated.example.com"
	if allPools[0].Group != "inference.networking.k8s.io" {
		t.Fatalf("CandidatePoolGVKs exposed input slice, got %v", allPools)
	}
}

func TestSupportedInferenceGroups(t *testing.T) {
	t.Parallel()

	if !IsSupportedInferenceObjectiveGroup("inference.networking.k8s.io") {
		t.Fatal("expected GA InferenceObjective group to be supported")
	}
	if IsSupportedInferenceObjectiveGroup("unsupported.example.com") {
		t.Fatal("expected unsupported InferenceObjective group to be rejected")
	}
	if !IsSupportedInferencePoolGroup("inference.networking.x-k8s.io") {
		t.Fatal("expected x-k8s InferencePool group to be supported")
	}
	if IsSupportedInferencePoolGroup("unsupported.example.com") {
		t.Fatal("expected unsupported InferencePool group to be rejected")
	}
}
