---
title: Identity Boundaries
weight: 15
aliases:
  - /operator/design/identity-boundaries/
---

## Boundaries

| Mode | Boundary |
| --- | --- |
| `PoolOnly` | One SPIFFE identity represents the serving pool pods. |
| `PerObjective` | One SPIFFE identity represents a GAIE `InferenceObjective`, scoped through the referenced pool. |

The pool defines where inference runs. The objective defines what is served.

## Container Discriminator

`PerObjective` uses a container discriminator to add a container-level selector
to the pool-level pod selection.

| Type | SPIRE selector | Notes |
| --- | --- | --- |
| `ContainerName` | `k8s:container-name:<value>` | Preferred. |
| `ContainerImage` | `k8s:container-image:<value>` | Weaker fallback because one image can serve multiple objectives. |

When multiple objectives share one pool, each objective should use a different
container name. If two `PerObjective` bindings resolve to the same pod set and
same container-name value, `kleym-operator` refuses both with reason
`IdentityCollision`.

Multiple `ClusterSPIFFEID` resources can select the same pod set. The controller
detects deterministic collisions between managed `PerObjective` bindings.
