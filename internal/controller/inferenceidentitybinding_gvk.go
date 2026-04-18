package controller

import "k8s.io/apimachinery/pkg/runtime/schema"

func (r *InferenceIdentityBindingReconciler) watchObjectiveGVKs() []schema.GroupVersionKind {
	if r.availableObjectiveGVKs != nil {
		return r.availableObjectiveGVKs
	}
	return inferenceObjectiveGVKs
}

func (r *InferenceIdentityBindingReconciler) resolveObjectiveGVKs() []schema.GroupVersionKind {
	if len(r.availableObjectiveGVKs) > 0 {
		return r.availableObjectiveGVKs
	}
	return inferenceObjectiveGVKs
}

func (r *InferenceIdentityBindingReconciler) resolvePoolGVKs() []schema.GroupVersionKind {
	if len(r.availablePoolGVKs) > 0 {
		return r.availablePoolGVKs
	}
	return inferencePoolGVKs
}
