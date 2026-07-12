package spirecm

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

func TestDesiredClusterSPIFFEIDIncludesHintAndFallback(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	binding.Name = "binding-a"
	binding.Namespace = "default"
	binding.Spec.IdentityBoundary = kleymv1alpha1.IdentityBoundary{
		Variant: "prefill",
	}
	plan := identity.Plan{
		SpiffeID:       "spiffe://kleym.sonda.red/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill",
		PodSelector:    map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:      []string{"k8s:ns:default", "k8s:sa:inference-sa", "k8s:pod-label:app:model-server", "k8s:pod-label:identity.kleym.sonda.red/variant:prefill"},
		IdentityAnchor: identity.IdentityAnchor{Kind: "pool", Name: "pool-a"},
		Variant:        "prefill",
	}

	desired := DesiredClusterSPIFFEID(binding, plan, "")
	spec, found, err := unstructured.NestedMap(desired.Object, "spec")
	if err != nil {
		t.Fatalf("failed to inspect desired ClusterSPIFFEID spec: %v", err)
	}
	if !found {
		t.Fatal("desired ClusterSPIFFEID spec missing")
	}
	if spec["hint"] != "default/binding-a" {
		t.Fatalf("hint = %q, want %q", spec["hint"], "default/binding-a")
	}
	if spec["fallback"] != false {
		t.Fatalf("fallback = %v, want false", spec["fallback"])
	}
}

func TestDesiredClusterSPIFFEIDUsesCanonicalSelectorTemplates(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	binding.Name = "binding-a"
	binding.Namespace = "default"
	binding.Spec.ServiceAccountName = "inference-sa"
	binding.Spec.IdentityBoundary = kleymv1alpha1.IdentityBoundary{
		Variant: "prefill",
	}
	plan, err := identity.PlanIdentity(identity.PlanInput{
		Namespace:          binding.Namespace,
		ServiceAccountName: binding.Spec.ServiceAccountName,
		TrustDomain:        "example.org",
		Variant:            "prefill",
		Target: identity.ResolvedInferenceTarget{
			IdentityAnchor: identity.IdentityAnchor{Kind: "pool", Name: "pool-a"},
			PodSelector:    map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
			DerivedSelectors: []string{
				"k8s:pod-label:team:ml",
				"k8s:pod-label:app:model-server",
				"k8s:pod-label:app:model-server",
			},
		},
	})
	if err != nil {
		t.Fatalf("PlanIdentity returned error: %v", err)
	}

	desired := DesiredClusterSPIFFEID(binding, plan, "")
	gotSelectors, found, err := unstructured.NestedStringSlice(
		desired.Object,
		"spec",
		"workloadSelectorTemplates",
	)
	if err != nil {
		t.Fatalf("failed to inspect workloadSelectorTemplates: %v", err)
	}
	if !found {
		t.Fatal("workloadSelectorTemplates missing")
	}

	wantSelectors := []string{
		"k8s:ns:default",
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:identity.kleym.sonda.red/variant:prefill",
		"k8s:pod-label:team:ml",
		"k8s:sa:inference-sa",
	}
	if !slices.Equal(gotSelectors, wantSelectors) {
		t.Fatalf("workloadSelectorTemplates = %v, want %v", gotSelectors, wantSelectors)
	}
}

func TestDesiredClusterSPIFFEIDClassName(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	binding.Name = "binding-a"
	binding.Namespace = "default"
	binding.Spec.IdentityBoundary = kleymv1alpha1.IdentityBoundary{
		Variant: "prefill",
	}
	plan := identity.Plan{
		SpiffeID:       "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill",
		PodSelector:    map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:      []string{"k8s:ns:default", "k8s:sa:inference-sa", "k8s:pod-label:app:model-server", "k8s:pod-label:identity.kleym.sonda.red/variant:prefill"},
		IdentityAnchor: identity.IdentityAnchor{Kind: "pool", Name: "pool-a"},
		Variant:        "prefill",
	}

	classless := DesiredClusterSPIFFEID(binding, plan, "")
	classlessSpec, _, err := unstructured.NestedMap(classless.Object, "spec")
	if err != nil {
		t.Fatalf("failed to inspect classless ClusterSPIFFEID spec: %v", err)
	}
	if _, ok := classlessSpec["className"]; ok {
		t.Fatalf("classless spec rendered className: %#v", classlessSpec["className"])
	}

	classed := DesiredClusterSPIFFEID(binding, plan, "kleym")
	classedSpec, _, err := unstructured.NestedMap(classed.Object, "spec")
	if err != nil {
		t.Fatalf("failed to inspect classed ClusterSPIFFEID spec: %v", err)
	}
	if classedSpec["className"] != "kleym" {
		t.Fatalf("className = %q, want kleym", classedSpec["className"])
	}
}

func TestOwnershipClaimAnnotationIsCreateProvenance(t *testing.T) {
	t.Parallel()

	current := &unstructured.Unstructured{}
	current.SetAnnotations(map[string]string{"example.test/preserved": "true"})
	SetClusterSPIFFEIDOwnershipClaim(current, "claim-created")
	if got := ClusterSPIFFEIDOwnershipClaim(current); got != "claim-created" {
		t.Fatalf("ownership claim = %q, want claim-created", got)
	}

	desired := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"spiffeIDTemplate": "spiffe://example.org/workload"}}}
	desired.SetAnnotations(map[string]string{OwnershipClaimIDAnnotationKey: "claim-replacement"})
	MergeDesiredClusterSPIFFEID(current, desired)
	if got := ClusterSPIFFEIDOwnershipClaim(current); got != "claim-created" {
		t.Fatalf("ownership claim after desired merge = %q, want immutable claim-created", got)
	}
	if current.GetAnnotations()["example.test/preserved"] != "true" {
		t.Fatalf("unrelated annotation was not preserved: %v", current.GetAnnotations())
	}
}

func TestBuildClusterSPIFFEIDNameUsesServiceAccountScopedIdentityHash(t *testing.T) {
	t.Parallel()

	got := BuildClusterSPIFFEIDName(
		"kleym-reference-inference",
		"binding",
		"pool",
		"spiffe://kleym.sonda.red/ns/kleym-reference-inference/sa/reference-inference/inference/pool/reference-pool/variant/reference",
	)
	want := "kleym-kleym-reference-inference-binding-pool-18b7ab03"
	if got != want {
		t.Fatalf("BuildClusterSPIFFEIDName = %q, want %q", got, want)
	}
}

func TestBuildClusterSPIFFEIDNameUsesAnchorKindSegment(t *testing.T) {
	t.Parallel()

	got := BuildClusterSPIFFEIDName(
		"default",
		"binding",
		"Model Endpoint",
		"spiffe://example.org/ns/default/sa/inference-sa/inference/model-endpoint/model-a",
	)
	if wantSegment := "-model-endpoint-"; !strings.Contains(got, wantSegment) {
		t.Fatalf("BuildClusterSPIFFEIDName = %q, want sanitized anchor kind segment %q", got, wantSegment)
	}
}

func TestBuildClusterSPIFFEIDNameFallsBackForEmptyAnchorKind(t *testing.T) {
	t.Parallel()

	got := BuildClusterSPIFFEIDName(
		"default",
		"binding",
		" ",
		"spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill",
	)
	if wantSegment := "-identity-"; !strings.Contains(got, wantSegment) {
		t.Fatalf("BuildClusterSPIFFEIDName = %q, want fallback anchor kind segment %q", got, wantSegment)
	}
}
