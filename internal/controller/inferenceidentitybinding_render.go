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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/identity"
)

func (r *InferenceIdentityBindingReconciler) renderIdentity(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	pool *unstructured.Unstructured,
) (identity.RenderedIdentity, error) {
	podSelector, poolDerivedSelectors, err := gaie.DeriveSelectorsFromPool(pool)
	if err != nil {
		return identity.RenderedIdentity{}, &identity.StateError{
			ConditionType: identity.ConditionTypeUnsafeSelector,
			Reason:        identity.ReasonInvalidPoolSelector,
			Message:       err.Error(),
		}
	}
	return identity.PlanIdentity(identity.PlanInput{
		Binding:              binding,
		TrustDomain:          r.Config.TrustDomain,
		PoolName:             pool.GetName(),
		PodSelector:          podSelector,
		PoolDerivedSelectors: poolDerivedSelectors,
	})
}

func (r *InferenceIdentityBindingReconciler) renderIdentityForBinding(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (identity.RenderedIdentity, error) {
	poolRef, err := gaie.BindingPoolRef(binding)
	if err != nil {
		return identity.RenderedIdentity{}, err
	}

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return identity.RenderedIdentity{}, err
	}

	return r.renderIdentity(binding, pool)
}
