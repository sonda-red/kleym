package controller

import (
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
		},
	}

	initializeConditions(&status, 7)

	canonicalConditionTypes := []string{
		conditionTypeReady,
		conditionTypeConflict,
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

	conflict := meta.FindStatusCondition(status.Conditions, conditionTypeConflict)
	if conflict == nil {
		t.Fatalf("expected condition %q to be present", conditionTypeConflict)
	}
	if conflict.Status != metav1.ConditionUnknown {
		t.Fatalf("conflict status = %q, want %q", conflict.Status, metav1.ConditionUnknown)
	}
	if conflict.Reason != "Initializing" {
		t.Fatalf("conflict reason = %q, want %q", conflict.Reason, "Initializing")
	}
}
