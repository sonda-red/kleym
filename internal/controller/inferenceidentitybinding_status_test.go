package controller

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

func TestInitializeConditionsEnsuresCanonicalSetForCurrentGeneration(t *testing.T) {
	t.Parallel()

	status := kleymv1alpha1.InferenceIdentityBindingStatus{
		Conditions: []metav1.Condition{
			{
				Type:               conditionTypeReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
				Reason:             "Reconciled",
				Message:            "Binding reconciled",
			},
			{
				Type:               "Conflict",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
				Reason:             "Obsolete",
				Message:            "stale obsolete condition from older controller",
			},
		},
	}

	initializeConditions(&status, 7)

	canonicalConditionTypes := []string{
		conditionTypeReady,
		conditionTypeInvalidRef,
		conditionTypeUnsafeSelector,
		conditionTypeRenderFailure,
	}

	for _, conditionType := range canonicalConditionTypes {
		condition := meta.FindStatusCondition(status.Conditions, conditionType)
		if condition == nil {
			t.Fatalf("expected condition %q to be present", conditionType)
		}
		if condition.ObservedGeneration != 7 {
			t.Fatalf("condition %q observedGeneration = %d, want 7", conditionType, condition.ObservedGeneration)
		}
	}

	ready := meta.FindStatusCondition(status.Conditions, conditionTypeReady)
	if ready == nil {
		t.Fatalf("expected condition %q to be present", conditionTypeReady)
	}
	if ready.Status != metav1.ConditionTrue {
		t.Fatalf("ready status = %q, want %q", ready.Status, metav1.ConditionTrue)
	}
	if ready.Reason != "Reconciled" {
		t.Fatalf("ready reason = %q, want %q", ready.Reason, "Reconciled")
	}
	if ready.Message != "Binding reconciled" {
		t.Fatalf("ready message = %q, want %q", ready.Message, "Binding reconciled")
	}

	conflict := meta.FindStatusCondition(status.Conditions, "Conflict")
	if conflict != nil {
		t.Fatalf("stale Conflict condition was not removed: %#v", conflict)
	}
}

func TestInitializeConditionsSetsUnevaluatedConditionsToInitializing(t *testing.T) {
	t.Parallel()

	status := kleymv1alpha1.InferenceIdentityBindingStatus{}

	initializeConditions(&status, 3)

	for _, conditionType := range []string{
		conditionTypeReady,
		conditionTypeInvalidRef,
		conditionTypeUnsafeSelector,
		conditionTypeRenderFailure,
	} {
		condition := meta.FindStatusCondition(status.Conditions, conditionType)
		if condition == nil {
			t.Fatalf("expected condition %q to be present", conditionType)
		}
		if condition.Status != metav1.ConditionUnknown {
			t.Fatalf("condition %q status = %q, want %q", conditionType, condition.Status, metav1.ConditionUnknown)
		}
		if condition.Reason != conditionReasonInitializing {
			t.Fatalf("condition %q reason = %q, want %q", conditionType, condition.Reason, conditionReasonInitializing)
		}
		if condition.ObservedGeneration != 3 {
			t.Fatalf("condition %q observedGeneration = %d, want 3", conditionType, condition.ObservedGeneration)
		}
	}
}

func TestApplySuccessStatusRecordsRenderedSelectors(t *testing.T) {
	t.Parallel()

	status := kleymv1alpha1.InferenceIdentityBindingStatus{}
	wantSelectors := []string{
		"k8s:ns:default",
		"k8s:pod-label:app:model-server",
		"k8s:sa:inference-sa",
	}

	spiffeID := "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a"
	applySuccessStatus(
		&status,
		4,
		[]renderedIdentity{{
			SpiffeID:  spiffeID,
			Selectors: wantSelectors,
		}},
		[]kleymv1alpha1.RenderedClusterSPIFFEIDStatus{{
			Name:                "kleym-default-binding-pool-34c1d5c4",
			SpiffeID:            spiffeID,
			SelectorFingerprint: identity.SelectorFingerprint(wantSelectors),
		}},
	)

	if len(status.RenderedSelectors) != 1 {
		t.Fatalf("renderedSelectors = %d, want 1", len(status.RenderedSelectors))
	}
	if !slices.Equal(status.RenderedSelectors[0].Selectors, wantSelectors) {
		t.Fatalf("selectors = %v, want %v", status.RenderedSelectors[0].Selectors, wantSelectors)
	}
	if status.RenderedClusterSPIFFEID == nil {
		t.Fatalf("renderedClusterSPIFFEID was not populated")
	}
	if status.RenderedClusterSPIFFEID.SpiffeID != status.ComputedSpiffeIDs[0].SpiffeID {
		t.Fatalf(
			"renderedClusterSPIFFEID.spiffeID = %q, want computed SPIFFE ID %q",
			status.RenderedClusterSPIFFEID.SpiffeID,
			status.ComputedSpiffeIDs[0].SpiffeID,
		)
	}
	if status.RenderedClusterSPIFFEID.SelectorFingerprint != identity.SelectorFingerprint(wantSelectors) {
		t.Fatalf("selectorFingerprint = %q, want sha256 fingerprint", status.RenderedClusterSPIFFEID.SelectorFingerprint)
	}
}

func TestApplyFailureStatusClearsRenderedManagedStatus(t *testing.T) {
	t.Parallel()

	generation := int64(3)
	status := kleymv1alpha1.InferenceIdentityBindingStatus{
		ComputedSpiffeIDs: []kleymv1alpha1.ComputedSpiffeIDStatus{{
			SpiffeID: "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a",
		}},
		RenderedSelectors: []kleymv1alpha1.RenderedSelectorStatus{{
			SpiffeID:  "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a",
			Selectors: []string{"k8s:ns:default"},
		}},
		RenderedClusterSPIFFEID: &kleymv1alpha1.RenderedClusterSPIFFEIDStatus{
			Name:                "kleym-default-binding-pool-34c1d5c4",
			SpiffeID:            "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a",
			SelectorFingerprint: "sha256:old",
			ObservedGeneration:  &generation,
		},
	}

	applyFailureStatus(&status, generation, newStateError(conditionTypeInvalidRef, "TargetPoolNotFound", "pool not found"))

	if len(status.ComputedSpiffeIDs) != 0 {
		t.Fatalf("computedSpiffeIDs = %d, want cleared", len(status.ComputedSpiffeIDs))
	}
	if len(status.RenderedSelectors) != 0 {
		t.Fatalf("renderedSelectors = %d, want cleared", len(status.RenderedSelectors))
	}
	if status.RenderedClusterSPIFFEID != nil {
		t.Fatalf("renderedClusterSPIFFEID = %#v, want cleared", status.RenderedClusterSPIFFEID)
	}
}

func TestRenderedClusterSPIFFEIDStatusRecordsObservedGeneration(t *testing.T) {
	t.Parallel()

	rendered := renderedIdentity{
		SpiffeID:  "spiffe://example.org/ns/default/sa/inference-sa/inference/pool/pool-a",
		Selectors: []string{"k8s:ns:default", "k8s:sa:inference-sa"},
	}
	object := &unstructured.Unstructured{}
	object.SetName("kleym-default-binding-pool-34c1d5c4")
	object.SetGeneration(7)

	status := renderedClusterSPIFFEIDStatus(rendered, object)

	if status.Name != object.GetName() {
		t.Fatalf("name = %q, want %q", status.Name, object.GetName())
	}
	if status.SpiffeID != rendered.SpiffeID {
		t.Fatalf("spiffeID = %q, want %q", status.SpiffeID, rendered.SpiffeID)
	}
	if status.SelectorFingerprint != identity.SelectorFingerprint(rendered.Selectors) {
		t.Fatalf("selectorFingerprint = %q, want fingerprint for rendered selectors", status.SelectorFingerprint)
	}
	if status.ObservedGeneration == nil || *status.ObservedGeneration != 7 {
		t.Fatalf("observedGeneration = %v, want 7", status.ObservedGeneration)
	}

	object.SetGeneration(0)
	status = renderedClusterSPIFFEIDStatus(rendered, object)
	if status.ObservedGeneration != nil {
		t.Fatalf("observedGeneration = %v, want omitted for zero generation", *status.ObservedGeneration)
	}
}
