/*
Copyright 2026 Kalin Daskalov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/prometheus/client_golang/prometheus"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

func TestDeriveBindingOutcomeLabelsReady(t *testing.T) {
	t.Parallel()

	binding := newPoolOnlyBinding("binding-ready", "")
	initializeConditions(&binding.Status, 1)
	applySuccessStatus(&binding.Status, binding.Generation, []renderedIdentity{{
		SpiffeID: "spiffe://kleym.sonda.red/ns/default/pool/pool-a",
	}})

	labels, ok := deriveBindingOutcomeLabels(binding, false)
	if !ok {
		t.Fatal("expected outcome labels")
	}
	if labels.condition != conditionTypeReady {
		t.Fatalf("condition = %q, want %q", labels.condition, conditionTypeReady)
	}
	if labels.reason != "Reconciled" {
		t.Fatalf("reason = %q, want %q", labels.reason, "Reconciled")
	}
}

func TestDeriveBindingOutcomeLabelsFailure(t *testing.T) {
	t.Parallel()

	binding := newPoolOnlyBinding("binding-failure", "")
	initializeConditions(&binding.Status, 1)
	applyFailureStatus(&binding.Status, binding.Generation, newStateError(
		conditionTypeRenderFailure,
		"InvalidServiceAccountName",
		"serviceAccountName is invalid",
	))

	labels, ok := deriveBindingOutcomeLabels(binding, false)
	if !ok {
		t.Fatal("expected outcome labels")
	}
	if labels.condition != conditionTypeRenderFailure {
		t.Fatalf("condition = %q, want %q", labels.condition, conditionTypeRenderFailure)
	}
	if labels.reason != "InvalidServiceAccountName" {
		t.Fatalf("reason = %q, want %q", labels.reason, "InvalidServiceAccountName")
	}
}

func TestDeriveBindingOutcomeLabelsCollision(t *testing.T) {
	t.Parallel()

	binding := newPoolOnlyBinding("binding-collision", "")
	initializeConditions(&binding.Status, 1)
	applyCollisionStatus(&binding.Status, binding.Generation, true, "identity collision with bindings default/peer")

	labels, ok := deriveBindingOutcomeLabels(binding, false)
	if !ok {
		t.Fatal("expected outcome labels")
	}
	if labels.condition != conditionTypeConflict {
		t.Fatalf("condition = %q, want %q", labels.condition, conditionTypeConflict)
	}
	if labels.reason != "IdentityCollision" {
		t.Fatalf("reason = %q, want %q", labels.reason, "IdentityCollision")
	}
}

func TestReconcileRecordsSuccessfulTerminalOutcomeAfterStatusPatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPoolOnlyBinding("binding-ready-metric", "")
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(newTestPool(), binding).
		Build()
	recorder := &persistedBindingOutcomeRecorder{
		client: k8sClient,
		key:    types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	}

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client:          k8sClient,
		Scheme:          scheme,
		MetricsRecorder: recorder,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recorder.key})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	if len(recorder.outcomes) != 1 {
		t.Fatalf("recorded outcomes = %d, want 1", len(recorder.outcomes))
	}
	outcome := recorder.outcomes[0]
	if outcome.condition != conditionTypeReady || outcome.reason != "Reconciled" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestReconcileRecordsFailureTerminalOutcomeAfterStatusPatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newCollisionTestScheme(t)

	binding := newPoolOnlyBinding("binding-failure-metric", "")
	binding.Spec.PoolRef.Name = "missing-pool"
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&kleymv1alpha1.InferenceIdentityBinding{}).
		WithObjects(binding).
		Build()
	recorder := &persistedBindingOutcomeRecorder{
		client: k8sClient,
		key:    types.NamespacedName{Namespace: testNamespace, Name: binding.Name},
	}

	reconciler := &InferenceIdentityBindingReconciler{Config: testOperatorConfig(),
		Client:          k8sClient,
		Scheme:          scheme,
		MetricsRecorder: recorder,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recorder.key})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	if len(recorder.outcomes) != 1 {
		t.Fatalf("recorded outcomes = %d, want 1", len(recorder.outcomes))
	}
	outcome := recorder.outcomes[0]
	if outcome.condition != conditionTypeInvalidRef || outcome.reason != "TargetPoolNotFound" {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestIdentityBindingGaugeCollectorAggregatesOutcomes(t *testing.T) {
	t.Parallel()

	scheme := newCollisionTestScheme(t)

	readyA := newPoolOnlyBinding("binding-ready-a", "")
	readyB := newPoolOnlyBinding("binding-ready-b", "")
	collision := newPoolOnlyBinding("binding-collision", "")
	initializing := newPoolOnlyBinding("binding-initializing", "")

	initializeConditions(&readyA.Status, 1)
	initializeConditions(&readyB.Status, 1)
	initializeConditions(&collision.Status, 1)
	initializeConditions(&initializing.Status, 1)

	applySuccessStatus(&readyA.Status, readyA.Generation, []renderedIdentity{{SpiffeID: "spiffe://kleym.sonda.red/ns/default/pool/pool-a"}})
	applySuccessStatus(&readyB.Status, readyB.Generation, []renderedIdentity{{SpiffeID: "spiffe://kleym.sonda.red/ns/default/pool/pool-a"}})
	applyCollisionStatus(&collision.Status, collision.Generation, true, "identity collision with bindings default/peer")

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(readyA, readyB, collision, initializing).
		Build()

	collector := newIdentityBindingGaugeCollector()
	collector.setListFunc(func(ctx context.Context) ([]kleymv1alpha1.InferenceIdentityBinding, error) {
		bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
		if err := k8sClient.List(ctx, bindingList); err != nil {
			return nil, err
		}
		return bindingList.Items, nil
	})

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	values := gatherMetricLabels(t, families, metricNameIdentityBindings)
	assertMetricValue(t, values, bindingOutcomeLabels{
		condition: conditionTypeReady,
		reason:    "Reconciled",
	}, 2)
	assertMetricValue(t, values, bindingOutcomeLabels{
		condition: conditionTypeConflict,
		reason:    "IdentityCollision",
	}, 1)
	assertMetricValue(t, values, bindingOutcomeLabels{
		condition: conditionTypeReady,
		reason:    metricReasonInitializing,
	}, 1)
}

type persistedBindingOutcomeRecorder struct {
	client   client.Client
	key      types.NamespacedName
	outcomes []bindingOutcomeLabels
}

func (r *persistedBindingOutcomeRecorder) RecordTerminalOutcome(binding *kleymv1alpha1.InferenceIdentityBinding) {
	current := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := r.client.Get(context.Background(), r.key, current); err != nil {
		panic(err)
	}

	outcome, ok := deriveBindingOutcomeLabels(current, false)
	if !ok {
		panic("expected persisted terminal outcome")
	}
	r.outcomes = append(r.outcomes, outcome)
}

func gatherMetricLabels(t *testing.T, families []*dto.MetricFamily, metricName string) map[bindingOutcomeLabels]float64 {
	t.Helper()

	values := map[bindingOutcomeLabels]float64{}
	for _, family := range families {
		if family.GetName() != metricName {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := bindingOutcomeLabels{}
			for _, pair := range metric.GetLabel() {
				switch pair.GetName() {
				case metricLabelCondition:
					labels.condition = pair.GetValue()
				case metricLabelReason:
					labels.reason = pair.GetValue()
				}
			}
			values[labels] = metric.GetGauge().GetValue()
		}
	}
	return values
}

func assertMetricValue(t *testing.T, values map[bindingOutcomeLabels]float64, labels bindingOutcomeLabels, want float64) {
	t.Helper()

	got, ok := values[labels]
	if !ok {
		t.Fatalf("missing metric for labels %+v", labels)
	}
	if got != want {
		t.Fatalf("metric value for %+v = %v, want %v", labels, got, want)
	}
}
