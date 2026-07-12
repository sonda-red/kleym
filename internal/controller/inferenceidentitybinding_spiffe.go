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
	statuses := make([]kleymv1alpha1.RenderedClusterSPIFFEIDStatus, 0, len(identities))
	for _, identity := range identities {
		desired := spirecm.DesiredClusterSPIFFEID(binding, identity, r.Config.ClusterSPIFFEIDClassName)
		status, err := r.reconcileClusterSPIFFEID(ctx, binding, identity, desired)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// reconcileClusterSPIFFEID applies one desired object only after the shared
// ownership classifier authorizes its exact claim or Kubernetes UID.
func (r *InferenceIdentityBindingReconciler) reconcileClusterSPIFFEID(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
	desired *unstructured.Unstructured,
) (kleymv1alpha1.RenderedClusterSPIFFEIDStatus, error) {
	observation, err := r.observeManagedClusterSPIFFEID(ctx, binding, desired.GetName())
	if err != nil {
		return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
	}

	current, err := r.authorizeManagedOutputApply(ctx, binding, desired, observation)
	if err != nil {
		return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
	}
	if current == nil {
		return r.createManagedClusterSPIFFEID(ctx, binding, identity, desired, observation)
	}
	return r.updateManagedClusterSPIFFEID(ctx, identity, current, desired)
}

// authorizeManagedOutputApply converts an ownership observation into either an
// exact live object that may be reconciled or an absent slot that may be created.
func (r *InferenceIdentityBindingReconciler) authorizeManagedOutputApply(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	desired *unstructured.Unstructured,
	observation managedOutputObservation,
) (*unstructured.Unstructured, error) {
	switch observation.kind {
	case unclaimedOutputAbsent, pendingOutputAbsent:
		return nil, nil
	case pendingOutputMatched:
		if err := r.confirmClusterSPIFFEIDOwnership(ctx, binding, observation.object.GetName(), observation.object.GetUID()); err != nil {
			return nil, err
		}
		return observation.object, nil
	case confirmedOutputMatched:
		return observation.object, nil
	case confirmedOutputAbsent:
		if err := r.clearClusterSPIFFEIDOwnership(ctx, binding); err != nil {
			return nil, err
		}
		return nil, nil
	case unclaimedOutputForeign, pendingOutputForeign, confirmedOutputForeign:
		if observation.kind != unclaimedOutputForeign {
			if err := r.clearClusterSPIFFEIDOwnership(ctx, binding); err != nil {
				return nil, err
			}
		}
		return nil, foreignManagedOutputError(desired.GetName(), observation)
	default:
		return nil, fmt.Errorf("unknown ClusterSPIFFEID ownership observation for %q", desired.GetName())
	}
}

// createManagedClusterSPIFFEID carries the durable pending claim onto Create
// and confirms the API-assigned UID before returning rendered success.
func (r *InferenceIdentityBindingReconciler) createManagedClusterSPIFFEID(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
	desired *unstructured.Unstructured,
	observation managedOutputObservation,
) (kleymv1alpha1.RenderedClusterSPIFFEIDStatus, error) {
	claimID := observation.record.claimID
	if observation.kind != pendingOutputAbsent {
		var err error
		claimID, err = r.reserveClusterSPIFFEIDClaim(ctx, binding, desired.GetName())
		if err != nil {
			return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
		}
	}
	spirecm.SetClusterSPIFFEIDOwnershipClaim(desired, claimID)
	logf.FromContext(ctx).Info("creating managed ClusterSPIFFEID", logKeyClusterSPIFFEID, desired.GetName(), logKeySpiffeID, identity.SpiffeID)
	if err := r.Create(ctx, desired); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, fmt.Errorf("refusing same-name ClusterSPIFFEID %q after create race: %w", desired.GetName(), err)
		}
		return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
	}
	if desired.GetUID() == "" {
		return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, fmt.Errorf("created ClusterSPIFFEID %q has no Kubernetes UID", desired.GetName())
	}
	if err := r.confirmClusterSPIFFEIDOwnership(ctx, binding, desired.GetName(), desired.GetUID()); err != nil {
		return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
	}
	return renderedClusterSPIFFEIDStatus(identity, desired), nil
}

// updateManagedClusterSPIFFEID corrects drift only for an object whose UID was
// authorized by confirmed status in authorizeManagedOutputApply.
func (r *InferenceIdentityBindingReconciler) updateManagedClusterSPIFFEID(
	ctx context.Context,
	identity renderedIdentity,
	current *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) (kleymv1alpha1.RenderedClusterSPIFFEIDStatus, error) {
	logger := logf.FromContext(ctx)
	if !spirecm.ClusterSPIFFEIDInSync(current, desired) {
		logger.Info("updating drifted managed ClusterSPIFFEID", logKeyClusterSPIFFEID, desired.GetName(), logKeySpiffeID, identity.SpiffeID)
		spirecm.MergeDesiredClusterSPIFFEID(current, desired)
		if err := r.Update(ctx, current); err != nil {
			return kleymv1alpha1.RenderedClusterSPIFFEIDStatus{}, err
		}
	} else {
		logger.V(1).Info("managed ClusterSPIFFEID already in sync", logKeyClusterSPIFFEID, desired.GetName(), logKeySpiffeID, identity.SpiffeID)
	}
	return renderedClusterSPIFFEIDStatus(identity, current), nil
}

// foreignManagedOutputError explains the exact ownership proof that failed
// while preserving the existing ManagedOutputApplyFailed public taxonomy.
func foreignManagedOutputError(name string, observation managedOutputObservation) error {
	switch observation.kind {
	case pendingOutputForeign:
		return fmt.Errorf("refusing same-name ClusterSPIFFEID %q: live ownership claim does not match the durable pending claim", name)
	case confirmedOutputForeign:
		return fmt.Errorf("refusing same-name ClusterSPIFFEID %q: live UID %q does not match confirmed UID %q", name, observation.object.GetUID(), observation.record.uid)
	default:
		return fmt.Errorf("refusing pre-existing ClusterSPIFFEID %q without durable ownership status", name)
	}
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
			if err := r.clearClusterSPIFFEIDOwnership(ctx, binding); err != nil {
				return false, err
			}
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
	observation, err := r.observeManagedClusterSPIFFEID(ctx, binding, "")
	if err != nil {
		return nil, err
	}
	switch observation.kind {
	case pendingOutputMatched:
		if err := r.confirmClusterSPIFFEIDOwnership(ctx, binding, observation.object.GetName(), observation.object.GetUID()); err != nil {
			return nil, err
		}
		return []*unstructured.Unstructured{observation.object}, nil
	case confirmedOutputMatched:
		return []*unstructured.Unstructured{observation.object}, nil
	case pendingOutputAbsent, pendingOutputForeign, confirmedOutputAbsent, confirmedOutputForeign:
		if err := r.clearClusterSPIFFEIDOwnership(ctx, binding); err != nil {
			return nil, err
		}
	}
	return nil, nil
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
		uid := object.GetUID()
		if err := r.Delete(ctx, object, client.Preconditions{UID: &uid}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
