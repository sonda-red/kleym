package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/sonda-red/kleym/internal/gaie"
)

func (r *InferenceIdentityBindingReconciler) resolveObjectiveGVKs() []schema.GroupVersionKind {
	return gaie.ResolveObjectiveGVKs(r.availableObjectiveGVKs)
}

func (r *InferenceIdentityBindingReconciler) resolvePoolGVKs() []schema.GroupVersionKind {
	return gaie.ResolvePoolGVKs(r.availablePoolGVKs)
}
