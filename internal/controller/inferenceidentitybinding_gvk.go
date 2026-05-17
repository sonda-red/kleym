package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/sonda-red/kleym/internal/identity"
)

func (r *InferenceIdentityBindingReconciler) resolveObjectiveGVKs() []schema.GroupVersionKind {
	return identity.ResolveObjectiveGVKs(r.availableObjectiveGVKs)
}

func (r *InferenceIdentityBindingReconciler) resolvePoolGVKs() []schema.GroupVersionKind {
	return identity.ResolvePoolGVKs(r.availablePoolGVKs)
}
