package identity

import (
	"errors"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestPlanIdentityRejectsInvalidServiceAccountName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"invalid-character":   "Invalid_ServiceAccount",
		"leading-whitespace":  " inference-sa",
		"trailing-whitespace": "inference-sa ",
	}

	for name, serviceAccountName := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			binding := testBinding()
			binding.Spec.ServiceAccountName = serviceAccountName

			_, err := PlanIdentity(testPlanInput(binding, "pool-a"))
			if err == nil {
				t.Fatalf("expected invalid service account error, got nil")
			}

			var stateErr *StateError
			if !errors.As(err, &stateErr) {
				t.Fatalf("expected StateError, got %T", err)
			}
			if stateErr.ConditionType != ConditionTypeRenderFailure || stateErr.Reason != "InvalidServiceAccountName" {
				t.Fatalf("condition/reason = %q/%q, want %q/InvalidServiceAccountName", stateErr.ConditionType, stateErr.Reason, ConditionTypeRenderFailure)
			}
		})
	}
}

func TestPlanIdentityUsesPoolSPIFFEID(t *testing.T) {
	t.Parallel()

	binding := testBinding()

	identity, err := PlanIdentity(testPlanInput(binding, "pool-a"))
	if err != nil {
		t.Fatalf("PlanIdentity returned error: %v", err)
	}

	expectedSPIFFEID := "spiffe://example.org/ns/default/pool/pool-a"
	if identity.SpiffeID != expectedSPIFFEID {
		t.Fatalf("spiffeID = %q, want %q", identity.SpiffeID, expectedSPIFFEID)
	}
	for _, expectedSelector := range []string{
		"k8s:ns:default",
		"k8s:sa:inference-sa",
		"k8s:pod-label:app:model-server",
	} {
		if !containsString(identity.Selectors, expectedSelector) {
			t.Fatalf("expected selector %q, selectors: %v", expectedSelector, identity.Selectors)
		}
	}
}

func TestPlanIdentityCanonicalizesRenderedSelectors(t *testing.T) {
	t.Parallel()

	input := testPlanInput(testBinding(), "pool-a")
	input.PoolDerivedSelectors = []string{
		"k8s:pod-label:z:last",
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:app:model-server",
		"k8s:sa:inference-sa",
		"k8s:pod-label:team:ml",
	}

	identity, err := PlanIdentity(input)
	if err != nil {
		t.Fatalf("PlanIdentity returned error: %v", err)
	}

	wantSelectors := []string{
		"k8s:ns:default",
		"k8s:pod-label:app:model-server",
		"k8s:pod-label:team:ml",
		"k8s:pod-label:z:last",
		"k8s:sa:inference-sa",
	}
	if !slices.Equal(identity.Selectors, wantSelectors) {
		t.Fatalf("selectors = %v, want %v", identity.Selectors, wantSelectors)
	}
}

func TestSelectorFingerprintUsesCanonicalSelectorSet(t *testing.T) {
	t.Parallel()

	left := SelectorFingerprint([]string{
		"k8s:sa:inference-sa",
		"k8s:pod-label:app:model-server",
		"k8s:ns:default",
		"k8s:pod-label:app:model-server",
	})
	right := SelectorFingerprint([]string{
		"k8s:ns:default",
		"k8s:pod-label:app:model-server",
		"k8s:sa:inference-sa",
	})

	if left != right {
		t.Fatalf("fingerprints differ for equivalent selector sets: %q != %q", left, right)
	}
	if left == SelectorFingerprint([]string{
		"k8s:ns:default",
		"k8s:pod-label:app:other",
		"k8s:sa:inference-sa",
	}) {
		t.Fatalf("fingerprint did not change for a different selector set")
	}
	if got, want := left[:7], "sha256:"; got != want {
		t.Fatalf("fingerprint prefix = %q, want %q", got, want)
	}
}

func TestPlanIdentityRejectsMissingTrustDomain(t *testing.T) {
	t.Parallel()

	binding := testBinding()
	input := testPlanInput(binding, "pool-a")
	input.TrustDomain = ""

	_, err := PlanIdentity(input)
	if err == nil {
		t.Fatalf("expected missing trust domain error, got nil")
	}
	var stateErr *StateError
	if !errors.As(err, &stateErr) {
		t.Fatalf("expected StateError, got %T", err)
	}
	if stateErr.ConditionType != ConditionTypeRenderFailure || stateErr.Reason != "MissingTrustDomain" {
		t.Fatalf("condition/reason = %q/%q, want %q/MissingTrustDomain", stateErr.ConditionType, stateErr.Reason, ConditionTypeRenderFailure)
	}
}

func TestPlanIdentityRejectsUnsafeSelector(t *testing.T) {
	t.Parallel()

	input := testPlanInput(testBinding(), "pool-a")
	input.PoolDerivedSelectors = append(input.PoolDerivedSelectors, "k8s:ns:other")

	_, err := PlanIdentity(input)
	if err == nil {
		t.Fatalf("expected unsafe selector error, got nil")
	}
	var stateErr *StateError
	if !errors.As(err, &stateErr) {
		t.Fatalf("expected StateError, got %T", err)
	}
	if stateErr.ConditionType != ConditionTypeUnsafeSelector || stateErr.Reason != "UnsafeSelector" {
		t.Fatalf("condition/reason = %q/%q, want %q/UnsafeSelector", stateErr.ConditionType, stateErr.Reason, ConditionTypeUnsafeSelector)
	}
}

func TestConditionTaxonomyReasonStrings(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		got  string
		want string
	}{
		"missing-trust-domain":         {got: ReasonMissingTrustDomain, want: "MissingTrustDomain"},
		"invalid-service-account-name": {got: ReasonInvalidServiceAccountName, want: "InvalidServiceAccountName"},
		"unsafe-selector":              {got: ReasonUnsafeSelector, want: "UnsafeSelector"},
		"invalid-spiffe-id":            {got: ReasonInvalidSPIFFEID, want: "InvalidSPIFFEID"},
		"invalid-pool-selector":        {got: ReasonInvalidPoolSelector, want: "InvalidPoolSelector"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("reason = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func testBinding() *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding-a",
			Namespace: "default",
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef:            kleymv1alpha1.InferencePoolTargetRef{Name: "pool-a"},
			ServiceAccountName: "inference-sa",
		},
	}
}

func testPlanInput(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	poolName string,
) PlanInput {
	return PlanInput{
		Binding:              binding,
		TrustDomain:          "example.org",
		PoolName:             poolName,
		PodSelector:          map[string]any{"matchLabels": map[string]any{"app": "model-server"}},
		PoolDerivedSelectors: []string{"k8s:pod-label:app:model-server"},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
