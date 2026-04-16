# Collision Detection

This page explains the current per-objective collision logic. The intended policy still lives in the [spec](../spec.md).

## Problem

Two `PerObjective` bindings can resolve to the same workload set if they point at the same pool and use the same container discriminator. In that case, distinct objective identities would land on the same container selection.

## Current Detection Strategy

For each `PerObjective` reconciliation, the controller builds a targeted candidate set instead of scanning every binding in the namespace:

- bindings with the same `containerDiscriminator` key (type plus value)
- plus previously colliding peers when the current binding is already in `Conflict=True`
- with a safe fallback to all `PerObjective` bindings if peer recovery data is unavailable

It then renders identities for that candidate set and computes a collision fingerprint from:

- the normalized pool-derived pod selector
- the normalized final selector set
- `containerDiscriminator.type`
- `containerDiscriminator.value`

Bindings with the same fingerprint are treated as colliding.

## Current Outcome

When a collision is detected:

- every colliding binding gets `Conflict=True`
- `Ready` is forced to `False`
- `computedSpiffeIDs` and `renderedSelectors` are cleared
- managed `ClusterSPIFFEID` resources for those bindings are deleted
- the controller emits an `IdentityCollision` warning event

When the collision is resolved:

- `Conflict` is reset to `False`
- normal reconciliation resumes
- the controller emits `IdentityCollisionResolved`

## Scope Boundary

The current collision path applies only to `PerObjective` bindings. `PoolOnly` bindings bypass that detection path.
