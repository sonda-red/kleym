---
title: Identity Boundaries
weight: 15
description: "Design rationale for Kleym identity boundaries across namespaces, service accounts, inference pools, objectives, and containers."
aliases:
  - /operator/design/identity-boundaries/
---

## Boundaries

| Mode | Boundary |
| --- | --- |
| `PoolOnly` | One SPIFFE identity represents the serving pool pods. |
| `PerObjective` | One SPIFFE identity represents a GAIE `InferenceObjective`, scoped through the referenced pool. |

The pool defines where inference runs. The objective defines what is served.

## Container Name

`PerObjective` uses `containerName` to add a container-level selector to the
pool-level pod selection.

| Field | SPIRE selector | Notes |
| --- | --- | --- |
| `containerName` | `k8s:container-name:<value>` | Required for `PerObjective`; forbidden for `PoolOnly`. |

When multiple objectives share one pool, each objective should use a different
container name. If two `PerObjective` bindings resolve to the same pod set and
same container-name value, `kleym-operator` refuses both with reason
`IdentityCollision`.

Multiple `ClusterSPIFFEID` resources can select the same pod set. The controller
detects deterministic collisions between managed `PerObjective` bindings.
