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

	"github.com/sonda-red/kleym/internal/gaie"
)

func (r *InferenceIdentityBindingReconciler) resolveInferencePool(
	ctx context.Context,
	poolRef gaie.PoolRef,
) (*unstructured.Unstructured, error) {
	return gaie.ResolveInferencePool(ctx, r.Client, r.resolvePoolGVKs(), poolRef)
}

func shouldCleanupManagedClusterSPIFFEIDs(conditionType string) bool {
	return conditionType == conditionTypeInvalidRef ||
		conditionType == conditionTypeUnsafeSelector ||
		conditionType == conditionTypeRenderFailure
}

func isInfrastructureNotReadyReason(reason string) bool {
	return reason == gaie.ReasonInferencePoolCRDMissing ||
		reason == conditionReasonClusterSPIFFEIDCRDMissing
}

// newClusterSPIFFEIDCRDMissingStateError keeps the managed-output dependency
// failure mapped to the RenderFailure taxonomy from docs/spec/operator.md.
func newClusterSPIFFEIDCRDMissingStateError() *reconcileStateError {
	return newStateError(
		conditionTypeRenderFailure,
		conditionReasonClusterSPIFFEIDCRDMissing,
		"ClusterSPIFFEID CRD is not installed",
	)
}
