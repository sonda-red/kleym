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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	identitypkg "github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func (r *InferenceIdentityBindingReconciler) reconcileClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identities []renderedIdentity,
) ([]kleymv1alpha1.RenderedClusterSPIFFEIDStatus, error) {
	logger := logf.FromContext(ctx)
	existing, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return nil, err
	}
	logger.V(1).Info("listed managed ClusterSPIFFEIDs", "count", len(existing))

	existingByName := make(map[string]*unstructured.Unstructured, len(existing))
	for _, item := range existing {
		existingByName[item.GetName()] = item
	}

	statuses := make([]kleymv1alpha1.RenderedClusterSPIFFEIDStatus, 0, len(identities))
	for _, identity := range identities {
		desired := spirecm.DesiredClusterSPIFFEID(binding, identity, r.Config.ClusterSPIFFEIDClassName)
		desiredName := desired.GetName()

		current, exists := existingByName[desiredName]
		if !exists {
			if err := r.reserveClusterSPIFFEIDName(ctx, binding, desiredName); err != nil {
				return nil, err
			}
		}
		if !exists {
			logger.Info(
				"creating managed ClusterSPIFFEID",
				logKeyClusterSPIFFEID, desiredName,
				logKeySpiffeID, identity.SpiffeID,
			)
			if err := r.Create(ctx, desired); err != nil {
				if apierrors.IsAlreadyExists(err) {
					return nil, fmt.Errorf(
						"refusing ClusterSPIFFEID %q not recorded in binding status: %w",
						desiredName,
						err,
					)
				}
				return nil, err
			}
			statuses = append(statuses, renderedClusterSPIFFEIDStatus(identity, desired))
			continue
		}

		if !spirecm.ClusterSPIFFEIDInSync(current, desired) {
			logger.Info(
				"updating drifted managed ClusterSPIFFEID",
				logKeyClusterSPIFFEID, desiredName,
				logKeySpiffeID, identity.SpiffeID,
			)
			spirecm.MergeDesiredClusterSPIFFEID(current, desired)
			if err := r.Update(ctx, current); err != nil {
				return nil, err
			}
			statuses = append(statuses, renderedClusterSPIFFEIDStatus(identity, current))
			continue
		}
		logger.V(1).Info(
			"managed ClusterSPIFFEID already in sync",
			logKeyClusterSPIFFEID, desiredName,
			logKeySpiffeID, identity.SpiffeID,
		)
		statuses = append(statuses, renderedClusterSPIFFEIDStatus(identity, current))
	}

	return statuses, nil
}

// cleanupChangedClusterSPIFFEIDName prevents a desired-name change from
// replacing the durable recorded name before the old output is gone.
func (r *InferenceIdentityBindingReconciler) cleanupChangedClusterSPIFFEIDName(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	desiredName string,
) (bool, error) {
	recordedName := recordedClusterSPIFFEIDName(binding)
	if recordedName == "" || recordedName == desiredName {
		return false, nil
	}

	logger := logf.FromContext(ctx)
	logger.Info(
		"cleaning up managed ClusterSPIFFEID before changing owned output name",
		logKeyClusterSPIFFEID, recordedName,
	)
	if err := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
		return false, err
	}
	remaining, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return false, err
	}
	if len(remaining) == 0 {
		if recordedClusterSPIFFEIDName(binding) != "" {
			statusBase := binding.DeepCopy()
			clearClusterSPIFFEIDOwnership(&binding.Status)
			clearedStatus := binding.DeepCopy().Status
			if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
				binding.Status = statusBase.Status
				return false, err
			}
			binding.Status = clearedStatus
		}
		return false, nil
	}

	logger.Info(
		"waiting for previous managed ClusterSPIFFEID to disappear before changing owned output name",
		logKeyClusterSPIFFEID, recordedName,
		logKeyRequeueAfter, deleteVerificationRequeueAfter,
	)
	return true, nil
}

// reserveClusterSPIFFEIDName durably claims an absent deterministic name before
// Create. A retry may trust an existing object only when this claim was stored.
func (r *InferenceIdentityBindingReconciler) reserveClusterSPIFFEIDName(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	desiredName string,
) error {
	if recordedClusterSPIFFEIDName(binding) == desiredName {
		return nil
	}
	if recordedName := recordedClusterSPIFFEIDName(binding); recordedName != "" {
		return fmt.Errorf("cannot reserve ClusterSPIFFEID %q while %q is recorded", desiredName, recordedName)
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(clusterSPIFFEIDGVK)
	getErr := r.Get(ctx, client.ObjectKey{Name: desiredName}, current)
	switch {
	case getErr == nil:
		return fmt.Errorf(
			"refusing pre-existing ClusterSPIFFEID %q not recorded in binding status",
			desiredName,
		)
	case !apierrors.IsNotFound(getErr):
		return getErr
	}

	statusBase := binding.DeepCopy()
	applyPendingManagedOutputClaimStatus(&binding.Status, binding.Generation, desiredName)
	claimedStatus := binding.DeepCopy().Status
	if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
		binding.Status = statusBase.Status
		return err
	}
	// Status().Patch updates binding from the API response, which does not include
	// evaluation fields that this intermediate merge did not persist. Keep the
	// complete in-memory evaluation for the final status patch in this reconcile.
	binding.Status = claimedStatus
	return nil
}

func renderedClusterSPIFFEIDStatus(
	identity renderedIdentity,
	object *unstructured.Unstructured,
) kleymv1alpha1.RenderedClusterSPIFFEIDStatus {
	return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{
		Name:                object.GetName(),
		SpiffeID:            identity.SpiffeID,
		SelectorFingerprint: identitypkg.SelectorFingerprint(identity.Selectors),
		ObservedGeneration:  observedGenerationStatus(object),
	}
}

func observedGenerationStatus(object *unstructured.Unstructured) *int64 {
	generation := object.GetGeneration()
	if generation <= 0 {
		return nil
	}
	return &generation
}

func (r *InferenceIdentityBindingReconciler) listManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]*unstructured.Unstructured, error) {
	name := recordedClusterSPIFFEIDName(binding)
	if name == "" {
		return nil, nil
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := r.Get(ctx, client.ObjectKey{Name: name}, object); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return []*unstructured.Unstructured{object}, nil
}

// recordedClusterSPIFFEIDName returns the durable pending or confirmed name
// that authorizes managed-output reconciliation and cleanup.
func recordedClusterSPIFFEIDName(binding *kleymv1alpha1.InferenceIdentityBinding) string {
	if binding.Status.OwnedClusterSPIFFEIDName != "" {
		return binding.Status.OwnedClusterSPIFFEIDName
	}
	return binding.Status.PendingClusterSPIFFEIDName
}

func clearClusterSPIFFEIDOwnership(status *kleymv1alpha1.InferenceIdentityBindingStatus) {
	status.PendingClusterSPIFFEIDName = ""
	status.OwnedClusterSPIFFEIDName = ""
}

func (r *InferenceIdentityBindingReconciler) cleanupManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	logger := logf.FromContext(ctx)
	objects, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return err
	}
	if len(objects) == 0 {
		logger.V(1).Info("no managed ClusterSPIFFEIDs to clean up")
	}
	for _, object := range objects {
		logger.Info("deleting managed ClusterSPIFFEID during cleanup", logKeyClusterSPIFFEID, object.GetName())
		if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
