package controller

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
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

	applySuccessStatus(&status, 4, []renderedIdentity{{
		SpiffeID:  "spiffe://example.org/ns/default/pool/pool-a",
		Selectors: wantSelectors,
	}})

	if len(status.RenderedSelectors) != 1 {
		t.Fatalf("renderedSelectors = %d, want 1", len(status.RenderedSelectors))
	}
	if !slices.Equal(status.RenderedSelectors[0].Selectors, wantSelectors) {
		t.Fatalf("selectors = %v, want %v", status.RenderedSelectors[0].Selectors, wantSelectors)
	}
}
