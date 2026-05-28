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

func TestBindingObjectiveRefNormalizesOptionalRef(t *testing.T) {
	t.Parallel()

	binding := &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
	}
	if ref, found, err := BindingObjectiveRef(binding); err != nil || found || ref != (ObjectiveRef{}) {
		t.Fatalf("BindingObjectiveRef without ref = (%+v, %v, %v), want zero false nil", ref, found, err)
	}

	binding.Spec.ObjectiveRef = &kleymv1alpha1.InferenceObjectiveTargetRef{
		Name:  " objective-a ",
		Group: " inference.networking.x-k8s.io ",
	}
	ref, found, err := BindingObjectiveRef(binding)
	if err != nil {
		t.Fatalf("BindingObjectiveRef returned error: %v", err)
	}
	if !found {
		t.Fatal("BindingObjectiveRef found = false, want true")
	}
	if ref != (ObjectiveRef{Name: "objective-a", Group: "inference.networking.x-k8s.io", Namespace: "default"}) {
		t.Fatalf("BindingObjectiveRef = %+v, want normalized objective ref", ref)
	}

	binding.Spec.ObjectiveRef.Group = "unsupported.example.com"
	if _, found, err := BindingObjectiveRef(binding); err == nil || !found {
		t.Fatalf("expected unsupported objective group error with found=true, got found=%v err=%v", found, err)
	}

	binding.Spec.ObjectiveRef.Group = ""
	binding.Spec.ObjectiveRef.Name = " "
	if _, found, err := BindingObjectiveRef(binding); err == nil || !found {
		t.Fatalf("expected missing objective name error with found=true, got found=%v err=%v", found, err)
	}
}
