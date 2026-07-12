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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/spirecm"
)

type managedOutputRecordKind int

const (
	managedOutputUnclaimed managedOutputRecordKind = iota
	managedOutputPending
	managedOutputConfirmed
)

type managedOutputObservationKind int

const (
	unclaimedOutputAbsent managedOutputObservationKind = iota
	unclaimedOutputForeign
	pendingOutputAbsent
	pendingOutputMatched
	pendingOutputForeign
	confirmedOutputAbsent
	confirmedOutputMatched
	confirmedOutputForeign
)

type managedOutputRecord struct {
	kind    managedOutputRecordKind
	name    string
	claimID string
	uid     types.UID
}

type managedOutputObservation struct {
	kind   managedOutputObservationKind
	record managedOutputRecord
	object *unstructured.Unstructured
}

// managedOutputRecordFromStatus returns the one durable ownership record that
// may authorize access to a ClusterSPIFFEID. Invalid partial records fail closed.
func managedOutputRecordFromStatus(binding *kleymv1alpha1.InferenceIdentityBinding) (managedOutputRecord, error) {
	pending := binding.Status.PendingClusterSPIFFEID
	owned := binding.Status.OwnedClusterSPIFFEID
	if pending != nil && owned != nil {
		return managedOutputRecord{}, fmt.Errorf("binding status records both pending and confirmed ClusterSPIFFEID ownership")
	}
	if pending != nil {
		if strings.TrimSpace(pending.Name) == "" || strings.TrimSpace(pending.ClaimID) == "" {
			return managedOutputRecord{}, fmt.Errorf("binding status has an incomplete pending ClusterSPIFFEID ownership record")
		}
		return managedOutputRecord{kind: managedOutputPending, name: pending.Name, claimID: pending.ClaimID}, nil
	}
	if owned != nil {
		if strings.TrimSpace(owned.Name) == "" || owned.UID == "" {
			return managedOutputRecord{}, fmt.Errorf("binding status has an incomplete confirmed ClusterSPIFFEID ownership record")
		}
		return managedOutputRecord{kind: managedOutputConfirmed, name: owned.Name, uid: owned.UID}, nil
	}
	return managedOutputRecord{kind: managedOutputUnclaimed}, nil
}

// observeManagedClusterSPIFFEID classifies the live object against durable
// status. Name, labels, desired spec, and binding provenance never authorize it.
func (r *InferenceIdentityBindingReconciler) observeManagedClusterSPIFFEID(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	unclaimedName string,
) (managedOutputObservation, error) {
	record, err := managedOutputRecordFromStatus(binding)
	if err != nil {
		return managedOutputObservation{}, err
	}
	lookupName := record.name
	if record.kind == managedOutputUnclaimed {
		lookupName = unclaimedName
	}
	if lookupName == "" {
		return classifyManagedClusterSPIFFEID(record, nil), nil
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	if err := r.Get(ctx, client.ObjectKey{Name: lookupName}, object); err != nil {
		if apierrors.IsNotFound(err) {
			return classifyManagedClusterSPIFFEID(record, nil), nil
		}
		return managedOutputObservation{}, err
	}
	return classifyManagedClusterSPIFFEID(record, object), nil
}

// classifyManagedClusterSPIFFEID is the ownership decision point shared by
// apply, replacement, conflict cleanup, validation cleanup, and finalization.
func classifyManagedClusterSPIFFEID(
	record managedOutputRecord,
	object *unstructured.Unstructured,
) managedOutputObservation {
	observation := managedOutputObservation{record: record, object: object}
	switch record.kind {
	case managedOutputPending:
		observation.kind = classifyPendingOutput(record, object)
	case managedOutputConfirmed:
		observation.kind = classifyConfirmedOutput(record, object)
	default:
		observation.kind = unclaimedOutputAbsent
		if object != nil {
			observation.kind = unclaimedOutputForeign
		}
	}
	return observation
}

func classifyPendingOutput(record managedOutputRecord, object *unstructured.Unstructured) managedOutputObservationKind {
	if object == nil {
		return pendingOutputAbsent
	}
	if object.GetName() == record.name && spirecm.ClusterSPIFFEIDOwnershipClaim(object) == record.claimID {
		return pendingOutputMatched
	}
	return pendingOutputForeign
}

func classifyConfirmedOutput(record managedOutputRecord, object *unstructured.Unstructured) managedOutputObservationKind {
	if object == nil {
		return confirmedOutputAbsent
	}
	if object.GetName() == record.name && object.GetUID() == record.uid {
		return confirmedOutputMatched
	}
	return confirmedOutputForeign
}

// reserveClusterSPIFFEIDClaim durably stores a fresh correlation token before
// Create. Retries reuse the same token until observed truth resolves the claim.
func (r *InferenceIdentityBindingReconciler) reserveClusterSPIFFEIDClaim(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	desiredName string,
) (string, error) {
	if record, err := managedOutputRecordFromStatus(binding); err != nil {
		return "", err
	} else if record.kind != managedOutputUnclaimed {
		return "", fmt.Errorf("cannot reserve ClusterSPIFFEID %q while %q is recorded", desiredName, record.name)
	}

	claimID := string(uuid.NewUUID())
	base := binding.DeepCopy()
	applyPendingManagedOutputStatus(&binding.Status, binding.Generation)
	binding.Status.PendingClusterSPIFFEID = &kleymv1alpha1.PendingClusterSPIFFEIDStatus{
		Name:    desiredName,
		ClaimID: claimID,
	}
	binding.Status.OwnedClusterSPIFFEID = nil
	if err := r.patchOwnershipStatus(ctx, base, binding); err != nil {
		return "", err
	}
	return claimID, nil
}

// confirmClusterSPIFFEIDOwnership persists the live UID before an existing
// claimed object may be updated or deleted.
func (r *InferenceIdentityBindingReconciler) confirmClusterSPIFFEIDOwnership(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	name string,
	uid types.UID,
) error {
	if uid == "" {
		return fmt.Errorf("cannot confirm ClusterSPIFFEID %q ownership without a Kubernetes UID", name)
	}
	base := binding.DeepCopy()
	binding.Status.PendingClusterSPIFFEID = nil
	binding.Status.OwnedClusterSPIFFEID = &kleymv1alpha1.OwnedClusterSPIFFEIDStatus{Name: name, UID: uid}
	return r.patchOwnershipStatus(ctx, base, binding)
}

// clearClusterSPIFFEIDOwnership persists proven absence of the recorded
// incarnation without making any claim about a same-name foreign object.
func (r *InferenceIdentityBindingReconciler) clearClusterSPIFFEIDOwnership(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	if binding.Status.PendingClusterSPIFFEID == nil && binding.Status.OwnedClusterSPIFFEID == nil {
		return nil
	}
	base := binding.DeepCopy()
	binding.Status.PendingClusterSPIFFEID = nil
	binding.Status.OwnedClusterSPIFFEID = nil
	return r.patchOwnershipStatus(ctx, base, binding)
}

// patchOwnershipStatus persists an ownership transition and its accompanying
// observed status from the exact resource version seen by the caller. Conflicts
// are retried by reconcile rather than overwriting an interleaving transition.
func (r *InferenceIdentityBindingReconciler) patchOwnershipStatus(
	ctx context.Context,
	base *kleymv1alpha1.InferenceIdentityBinding,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	target := base.DeepCopy()
	target.Status = binding.DeepCopy().Status
	patch := client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})
	if err := r.Status().Patch(ctx, target, patch); err != nil {
		binding.Status = base.Status
		return err
	}
	binding.SetResourceVersion(target.GetResourceVersion())
	return nil
}

// recordedClusterSPIFFEIDName returns the name from the one durable ownership record.
func recordedClusterSPIFFEIDName(binding *kleymv1alpha1.InferenceIdentityBinding) string {
	if binding.Status.OwnedClusterSPIFFEID != nil {
		return binding.Status.OwnedClusterSPIFFEID.Name
	}
	if binding.Status.PendingClusterSPIFFEID != nil {
		return binding.Status.PendingClusterSPIFFEID.Name
	}
	return ""
}
