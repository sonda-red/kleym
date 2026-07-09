---
title: Reconciliation
weight: 10
description: "Detailed reconciliation design for resolving inference inputs, rendering identities, applying ClusterSPIFFEID resources, and patching status."
aliases:
  - /operator/design/reconciliation/
---

## Current Flow

For each `InferenceIdentityBinding`, the reconciler currently does the following:

1. Fetch the binding and ensure the `kleym.sonda.red/inferenceidentitybinding-finalizer` is present.
2. Resolve the referenced `InferencePool` from `spec.poolRef`.
3. Resolve the pool to identity anchor `pool/<pool-name>` plus pod-label selector inputs.
4. Render namespace and service-account safety selectors from the binding and merge them with the resolved target selectors.
5. Validate selector safety before rendering or writing output.
6. Render the deterministic service-account-scoped inference target SPIFFE ID.
7. Create, update, or delete managed `ClusterSPIFFEID` resources to match the rendered identity.
8. Patch binding status and emit events for success or failure.

## Requeue Sources

The controller does not only react to the binding itself. It also watches:

- `InferencePool`, so selector changes requeue only bindings whose `spec.poolRef.name` points at that pool

That keeps the rendered identity tied to current pool state instead of only the binding object.

Watch predicates filter status-only update events to avoid hot loops. Create and delete events still enqueue, and update events enqueue when object generation changes or deletion state changes.

## Failure Shape

The reconciler treats invalid references, unsafe selectors, and render failures as controller state, not as crashes. In those paths it updates status, emits an event, and removes stale managed output instead of leaving outdated `ClusterSPIFFEID` resources behind.

For missing required external CRDs (`InferencePool` or `ClusterSPIFFEID`), the reconciler also schedules a timed retry (`RequeueAfter`) so recovery does not depend on unrelated future events.
