---
title: Collision Detection
weight: 30
description: "Collision detection design for preventing multiple PerObjective bindings from rendering identities onto the same workload slice."
aliases:
  - /operator/design/collision-detection/
---

## Problem

Two `PerObjective` bindings can resolve to the same workload set if they point at the same pool and use the same `containerName`. In that case, distinct objective identities would land on the same container selection.

## Current Detection Strategy

For each `PerObjective` reconciliation, the controller builds a targeted candidate set instead of scanning every binding in the namespace:

- bindings with the same `containerName`
- plus previously colliding peers when the current binding is already in `Conflict=True`
- with a safe fallback to all `PerObjective` bindings if peer recovery data is unavailable

The candidate lookup uses field indexes when available. If an index is not available (for example during envtest bootstrap before index registration, or in partial startup states), the controller falls back to listing bindings in the namespace and filtering in memory.

It then renders identities for that candidate set and computes a collision fingerprint from:

- the normalized pool-derived pod selector
- the normalized final selector set
- `containerName`

Bindings with the same fingerprint are treated as colliding.

To recover peer bindings across reconciliations, the controller stores peer names in the `Conflict` condition message. This avoids introducing an API status field dedicated to a rare collision path while still allowing deterministic peer recovery. If the message cannot be parsed, the controller falls back to scanning all `PerObjective` bindings in the namespace.

The parseable `Conflict=True` message format is compatibility-sensitive:

```text
identity collision with bindings <peer-name>[, <peer-name>...]: PerObjective bindings must not share the same pod selector and container name
```

The peer list excludes the current binding and is rendered in sorted order. Future wording changes must either preserve this parseable structure or intentionally update the parser, tests, and condition reference together. This is status recovery data, not a new user-authored API field.

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
