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
		Mode:        kleymv1alpha1.InferenceIdentityBindingModePoolOnly,
		SpiffeID:    "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
		PodSelector: map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		Selectors:   []string{"k8s:ns:default", "k8s:sa:inference-sa", "k8s:pod-label:app:model-server"},
	}

	desired := DesiredClusterSPIFFEID(binding, plan)
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
