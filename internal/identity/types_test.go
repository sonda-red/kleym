package identity

import "testing"

func TestStateErrorErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	err := &StateError{Message: "render failed"}
	if err.Error() != "render failed" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "render failed")
	}
}

func TestNamespacedBindingKey(t *testing.T) {
	t.Parallel()

	if got := NamespacedBindingKey("default", "binding-a"); got != "default/binding-a" {
		t.Fatalf("NamespacedBindingKey = %q, want %q", got, "default/binding-a")
	}
}
