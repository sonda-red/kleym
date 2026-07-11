---
title: Reconciliation
weight: 10
description: "Detailed reconciliation design for resolving inference inputs, rendering identities, applying ClusterSPIFFEID resources, and patching status."
aliases:
  - /operator/design/reconciliation/
---

## Current Flow

For each `InferenceIdentityBinding`, the reconciler currently does the following:

1. Fetch the binding. If it is deleting, remove recorded managed output, confirm
   absence, and only then remove the
   `kleym.sonda.red/inferenceidentitybinding-finalizer`.
2. Otherwise, ensure the finalizer and resolve the referenced `InferencePool`
   from `spec.poolRef`.
3. Resolve the pool to identity anchor `pool/<pool-name>` plus pod-label selector inputs.
4. Validate the required identity boundary and render the mandatory namespace,
   service-account, complete pool, and boundary selectors.
5. Render the deterministic service-account-scoped inference target SPIFFE ID.
6. Evaluate peer bindings using the pairwise exclusivity contract in the
   [Operator Spec](/spec/operator/#identity-boundary-and-exclusivity).
7. For a conflict or duplicate claim, withdraw managed output from every
   conflict member and confirm absence before settling `Conflict=True`.
8. For an exclusive claim, create or update the managed `ClusterSPIFFEID` only
   after any previously recorded output is confirmed absent.
9. Patch binding status and emit events for success or failure.

## Requeue Sources

The controller does not only react to the binding itself. It also watches:

- `InferencePool`, so selector changes requeue only bindings whose `spec.poolRef.name` points at that pool
- peer `InferenceIdentityBinding` objects, so binding creation, update, and
  deletion converge every affected peer without relying on in-memory state
- managed `ClusterSPIFFEID` objects, so deletion or drift requeues the binding
  that recorded the output name

That keeps the rendered identity tied to current pool state instead of only the binding object.

Watch predicates filter status-only update events to avoid hot loops. Create and delete events still enqueue, and update events enqueue when object generation changes or deletion state changes.

## Failure Shape

The reconciler treats invalid references, unsafe selectors, conflicts, and
render failures as controller state, not as crashes. Invalid boundaries report
`UnsafeSelector=True` with reason `InvalidIdentityBoundary`. Conflict members
report `Conflict=True` only after their managed output is absent; duplicate
SPIFFE ID claims use reason `DuplicateIdentityBinding`, while other boundary
conflicts use `IdentityBoundaryConflict`.

When old or conflicting output deletion is still converging, rendered output is
cleared and `Ready=Unknown` with reason `Initializing` records that absence has
not yet been confirmed. API uncertainty is not absence confirmation.

Controller setup fails when the supported `InferencePool` GVK is not served, so
installing that CRD requires restarting an operator that already failed startup.
After startup, a missing `ClusterSPIFFEID` CRD is a managed-output failure and
uses a timed retry (`RequeueAfter`) so recovery does not depend on unrelated
watch events.
