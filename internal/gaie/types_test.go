package gaie

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestBindingPoolRefNormalizesAndValidatesGroup(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef: kleymv1alpha1.InferencePoolTargetRef{
				Name:  " pool-a ",
				Group: " inference.networking.k8s.io ",
			},
		},
	}

	ref, err := BindingPoolRef(binding)
	if err != nil {
		t.Fatalf("BindingPoolRef returned error: %v", err)
	}
	if ref != (PoolRef{Name: "pool-a", Group: "inference.networking.k8s.io", Namespace: "default"}) {
		t.Fatalf("BindingPoolRef = %+v, want normalized pool ref", ref)
	}

	binding.Spec.PoolRef.Group = "unsupported.example.com"
	if _, err := BindingPoolRef(binding); err == nil {
		t.Fatal("expected unsupported pool group error, got nil")
	}

	binding.Spec.PoolRef.Group = ""
	binding.Spec.PoolRef.Name = " "
	if _, err := BindingPoolRef(binding); err == nil {
		t.Fatal("expected missing pool name error, got nil")
	}
}
