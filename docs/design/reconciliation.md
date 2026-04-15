# Reconciliation

This page explains the current controller flow. The behavior contract remains in the [spec](../spec.md).

## Current Flow

For each `InferenceIdentityBinding`, the reconciler currently does the following:

1. Fetch the binding and ensure the `kleym.sonda.red/inferenceidentitybinding-finalizer` is present.
2. Resolve the referenced `InferenceObjective` from `spec.targetRef.name`.
3. Extract `spec.poolRef` from that objective and reject cross-namespace references.
4. Resolve the referenced `InferencePool`.
5. Derive pod-label selectors from `pool.spec.selector`.
6. Render workload selector templates from the binding and merge them with the pool-derived selectors.
7. Add the container discriminator selector when the effective mode is `PerObjective`.
8. Validate selector safety before rendering or writing output.
9. Render the SPIFFE ID, using either the custom template or the built-in default for the mode.
10. Run per-objective collision detection.
11. Create, update, or delete managed `ClusterSPIFFEID` resources to match the rendered identity.
12. Patch binding status and emit events for success, conflict, or failure.

## Requeue Sources

The controller does not only react to the binding itself. It also watches:

- `InferenceObjective`, so changes to `poolRef` or object existence requeue affected bindings
- `InferencePool`, so selector changes requeue bindings whose objectives reference that pool

That keeps the rendered identity tied to current objective and pool state instead of only the binding object.

## Failure Shape

The reconciler treats invalid references, unsafe selectors, render failures, and collisions as controller state, not as crashes. In those paths it updates status, emits an event, and removes stale managed output instead of leaving outdated `ClusterSPIFFEID` resources behind.
