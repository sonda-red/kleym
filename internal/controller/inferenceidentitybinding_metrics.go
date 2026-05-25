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
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	metricNameIdentityBindingOutcomes = "kleym_identity_binding_outcomes_total"
	metricNameIdentityBindings        = "kleym_identity_bindings"
	metricLabelCondition              = "condition"
	metricLabelReason                 = "reason"
	metricLabelMode                   = "mode"
	metricReasonInitializing          = "Initializing"
)

var failureOutcomeConditionOrder = []string{
	conditionTypeConflict,
	conditionTypeInvalidRef,
	conditionTypeUnsafeSelector,
	conditionTypeRenderFailure,
}

var defaultBindingMetrics = newInferenceIdentityBindingMetrics()

type bindingOutcomeLabels struct {
	condition string
	reason    string
	mode      string
}

type bindingOutcomeRecorder interface {
	RecordTerminalOutcome(binding *kleymv1alpha1.InferenceIdentityBinding)
}

type prometheusBindingOutcomeRecorder struct {
	counter *prometheus.CounterVec
}

type inferenceIdentityBindingMetrics struct {
	outcomeRecorder *prometheusBindingOutcomeRecorder
	gaugeCollector  *identityBindingGaugeCollector
}

type bindingsListFunc func(context.Context) ([]kleymv1alpha1.InferenceIdentityBinding, error)

type identityBindingGaugeCollector struct {
	mu       sync.RWMutex
	desc     *prometheus.Desc
	listFunc bindingsListFunc
}

func newInferenceIdentityBindingMetrics() *inferenceIdentityBindingMetrics {
	return &inferenceIdentityBindingMetrics{
		outcomeRecorder: &prometheusBindingOutcomeRecorder{
			counter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: metricNameIdentityBindingOutcomes,
					Help: "Terminal reconcile outcomes for InferenceIdentityBinding resources.",
				},
				[]string{metricLabelCondition, metricLabelReason, metricLabelMode},
			),
		},
		gaugeCollector: newIdentityBindingGaugeCollector(),
	}
}

func newIdentityBindingGaugeCollector() *identityBindingGaugeCollector {
	return &identityBindingGaugeCollector{
		desc: prometheus.NewDesc(
			metricNameIdentityBindings,
			"Current InferenceIdentityBinding outcomes aggregated from object status.",
			[]string{metricLabelCondition, metricLabelReason, metricLabelMode},
			nil,
		),
	}
}

// register wires the public controller metrics into controller-runtime's shared
// registry and updates the scrape-time binding lister used by the gauge.
func (m *inferenceIdentityBindingMetrics) register(k8sClient client.Client) error {
	m.gaugeCollector.setListFunc(func(ctx context.Context) ([]kleymv1alpha1.InferenceIdentityBinding, error) {
		bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
		if err := k8sClient.List(ctx, bindingList); err != nil {
			return nil, err
		}
		return bindingList.Items, nil
	})

	if err := registerMetricsCollector(m.outcomeRecorder.counter); err != nil {
		return err
	}
	return registerMetricsCollector(m.gaugeCollector)
}

func registerMetricsCollector(collector prometheus.Collector) error {
	if err := metrics.Registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			return nil
		}
		return err
	}
	return nil
}

func (r *prometheusBindingOutcomeRecorder) RecordTerminalOutcome(binding *kleymv1alpha1.InferenceIdentityBinding) {
	labels, ok := deriveBindingOutcomeLabels(binding, false)
	if !ok {
		return
	}

	r.counter.WithLabelValues(labels.condition, labels.reason, labels.mode).Inc()
}

func (c *identityBindingGaugeCollector) setListFunc(listFunc bindingsListFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listFunc = listFunc
}

func (c *identityBindingGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *identityBindingGaugeCollector) Collect(ch chan<- prometheus.Metric) {
	listFunc := c.currentListFunc()
	if listFunc == nil {
		return
	}

	bindings, err := listFunc(context.Background())
	if err != nil {
		return
	}

	aggregated := make(map[bindingOutcomeLabels]float64)
	for i := range bindings {
		labels, ok := deriveBindingOutcomeLabels(&bindings[i], true)
		if !ok {
			continue
		}
		aggregated[labels]++
	}

	for labels, value := range aggregated {
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			value,
			labels.condition,
			labels.reason,
			labels.mode,
		)
	}
}

func (c *identityBindingGaugeCollector) currentListFunc() bindingsListFunc {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listFunc
}

// deriveBindingOutcomeLabels reduces a binding's status to one bounded outcome
// tuple so status conditions remain the per-object debugging surface while
// metrics expose only low-cardinality aggregate labels.
func deriveBindingOutcomeLabels(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	allowInitializing bool,
) (bindingOutcomeLabels, bool) {
	mode := string(effectiveMode(binding.Spec.Mode))
	ready := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeReady)
	if ready != nil {
		if ready.Status == metav1.ConditionTrue {
			return bindingOutcomeLabels{
				condition: conditionTypeReady,
				reason:    ready.Reason,
				mode:      mode,
			}, true
		}
		if allowInitializing && ready.Status == metav1.ConditionUnknown && ready.Reason == metricReasonInitializing {
			return bindingOutcomeLabels{
				condition: conditionTypeReady,
				reason:    metricReasonInitializing,
				mode:      mode,
			}, true
		}
	}

	for _, conditionType := range failureOutcomeConditionOrder {
		condition := meta.FindStatusCondition(binding.Status.Conditions, conditionType)
		if condition != nil && condition.Status == metav1.ConditionTrue {
			return bindingOutcomeLabels{
				condition: condition.Type,
				reason:    condition.Reason,
				mode:      mode,
			}, true
		}
	}

	return bindingOutcomeLabels{}, false
}

func (r *InferenceIdentityBindingReconciler) recordTerminalOutcome(binding *kleymv1alpha1.InferenceIdentityBinding) {
	if r.MetricsRecorder == nil {
		return
	}
	r.MetricsRecorder.RecordTerminalOutcome(binding)
}
