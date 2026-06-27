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
3. Resolve `spec.objectiveRef` when present or required by `PerObjective`.
4. Validate that any resolved objective points at the same pool as `spec.poolRef`.
5. Derive pod-label selectors from `pool.spec.selector`.
6. Render namespace and service-account safety selectors from the binding and merge them with the pool-derived selectors.
7. Add the `containerName` selector when the effective mode is `PerObjective`.
8. Validate selector safety before rendering or writing output.
9. Render the deterministic SPIFFE ID for the mode.
10. Run per-objective collision detection.
11. Create, update, or delete managed `ClusterSPIFFEID` resources to match the rendered identity.
12. Patch binding status and emit events for success, conflict, or failure.

## Requeue Sources

The controller does not only react to the binding itself. It also watches:

- `InferenceObjective`, so changes to object existence or `spec.poolRef` requeue only bindings whose `spec.objectiveRef.name` points at that objective
- `InferencePool`, so selector changes requeue only bindings whose `spec.poolRef.name` points at that pool

That keeps the rendered identity tied to current objective and pool state instead of only the binding object.

Watch predicates filter status-only update events to avoid hot loops. Create and delete events still enqueue, and update events enqueue when object generation changes or deletion state changes.

## Failure Shape

The reconciler treats invalid references, unsafe selectors, render failures, and collisions as controller state, not as crashes. In those paths it updates status, emits an event, and removes stale managed output instead of leaving outdated `ClusterSPIFFEID` resources behind.

For missing required external CRDs (`InferencePool`, `ClusterSPIFFEID`, or `InferenceObjective` when an objective subject is needed), the reconciler also schedules a timed retry (`RequeueAfter`) so recovery does not depend on unrelated future events.
