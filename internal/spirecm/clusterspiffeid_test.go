package spirecm

import (
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
	plan := identity.Plan{
		SpiffeID:    "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
		PodSelector: map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:   []string{"k8s:ns:default", "k8s:sa:inference-sa", "k8s:pod-label:app:model-server"},
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

func TestDesiredClusterSPIFFEIDClassName(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	binding.Name = "binding-a"
	binding.Namespace = "default"
	plan := identity.Plan{
		SpiffeID:    "spiffe://example.org/ns/default/pool/pool-a",
		PodSelector: map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:   []string{"k8s:ns:default", "k8s:sa:inference-sa", "k8s:pod-label:app:model-server"},
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
