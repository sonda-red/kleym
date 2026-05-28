package identity

import (
	"testing"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestStateErrorErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	err := &StateError{Message: "render failed"}
	if err.Error() != "render failed" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "render failed")
	}
}

func TestNamespacedBindingKeyAndEffectiveMode(t *testing.T) {
	t.Parallel()

	if got := NamespacedBindingKey("default", "binding-a"); got != "default/binding-a" {
		t.Fatalf("NamespacedBindingKey = %q, want %q", got, "default/binding-a")
	}
	if got := EffectiveMode(""); got != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		t.Fatalf("EffectiveMode(\"\") = %q, want %q", got, kleymv1alpha1.InferenceIdentityBindingModePerObjective)
	}
	if got := EffectiveMode(kleymv1alpha1.InferenceIdentityBindingModePoolOnly); got != kleymv1alpha1.InferenceIdentityBindingModePoolOnly {
		t.Fatalf("EffectiveMode(pool-only) = %q, want pool-only", got)
	}
}
