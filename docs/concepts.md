---
title: Concepts
weight: 10
---

The [spec](spec) remains the authoritative contract.

## What `InferenceIdentityBinding` Is

`InferenceIdentityBinding` is the namespaced intent object owned by `kleym`.

It tells the controller:

- which `InferenceObjective` should receive an identity
- whether identity should be scoped to the whole pool or to one objective
- which safety selectors must always be present
- how to narrow a per-objective identity down to one container when needed

`kleym` then resolves the objective's `poolRef`, derives workload selection from the referenced `InferencePool`, and writes one or more `ClusterSPIFFEID` resources.

## Identity Modes

`kleym` has two identity boundaries.

| Mode | Meaning | Typical use |
| --- | --- | --- |
| `PoolOnly` | One identity for the serving pool. | The whole pool is one workload boundary and model-level separation is not needed. |
| `PerObjective` | One identity per `InferenceObjective`. | Multiple objectives share a pool but still need distinct identities. |

`PoolOnly` answers "which serving pool is this pod part of?"

`PerObjective` answers "which model endpoint is this container serving?"

If you do not set `mode`, the controller defaults to `PerObjective`.

## Why Container Discrimination Exists

`PerObjective` only makes sense if `kleym` can prove that one objective identity lands on one workload slice.

When several objectives share the same `InferencePool`, the pod selector alone is not enough because every objective may resolve to the same pods. The container discriminator adds a narrower selector so the identity applies to the intended serving container instead of the whole pod.

Current discriminator types:

- `ContainerName`, which is preferred because container names are explicit and stable within a pod template
- `ContainerImage`, which is a fallback when container names are not a useful discriminator

Without this extra boundary, distinct objective identities could collapse onto the same workload selection.

## Safety Selectors

Safety selectors are the controller's proof that the rendered identity stays inside the intended tenant boundary.

Every rendered identity must include:

- the binding namespace selector
- the workload service account selector
- selectors derived from the referenced pool
- the container discriminator when the mode is `PerObjective`

`kleym` refuses to reconcile when it cannot prove those constraints. That is why selector handling is intentionally narrower than raw Kubernetes label selection.

## What `kleym` Does Not Do

`kleym` is an identity registration compiler.

It does not:

- deploy inference workloads
- create `InferencePool` or `InferenceObjective` objects
- route traffic
- configure gateways, meshes, or policy engines
- issue certificates itself

SPIRE and SPIRE Controller Manager remain responsible for issuing identities. `kleym` only determines which identities should exist and which workloads they should target.

## See Also

- Read [architecture](architecture) for the end-to-end control flow.
- Read [examples](examples/basic-binding) for concrete manifests.
- Read [spec](spec) if you need the full contract.
