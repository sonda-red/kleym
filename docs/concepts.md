---
title: Concepts
weight: 10
aliases:
  - /operator/concepts/
---

## GAIE Resources

- [`InferencePool` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool intent. `kleym-operator` resolves the pool named by `spec.poolRef.name` and derives selector input from `spec.selector`.
- [`InferenceObjective` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/): optional model-level serving intent. `kleym-operator` resolves it from `spec.objectiveRef.name` for `PerObjective` bindings and validates that its `spec.poolRef` points at the binding pool.
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical reference for GAIE resource schemas and status fields.
- [GAIE GA migration guide](https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/): background on migration from `InferenceModel` to `InferenceObjective`.

`kleym-operator` supports GAIE objects from both `inference.networking.k8s.io/v1` and `inference.networking.x-k8s.io/v1alpha2`. See [GAIE Compatibility](/reference/gaie-compatibility/) for the current supported GVK list.

## Identity Modes

| Mode | Meaning | Typical use |
| --- | --- | --- |
| `PoolOnly` | One identity for the serving pool. | The whole pool is one workload boundary and model-level separation is not needed. |
| `PerObjective` | One identity per referenced `InferenceObjective`. | Multiple objectives share a pool but still need distinct identities. |

`PoolOnly` answers "which serving pool is this pod part of?"

`PerObjective` answers "which model endpoint is this container serving?"

If you do not set `mode`, the controller defaults to `PerObjective`.

## Why Container Discrimination Exists

`PerObjective` only makes sense if `kleym-operator` can prove that one objective identity lands on one workload slice.

When several objectives share the same `InferencePool`, the pod selector alone is not enough because every objective may resolve to the same pods. The container discriminator adds a container selector so the identity applies to the intended serving container instead of the whole pod.

Current discriminator types:

- `ContainerName`, which is preferred because container names are explicit and stable within a pod template
- `ContainerImage`, which is a fallback when container names are unavailable

Without this extra boundary, distinct objective identities could collapse onto the same workload selection.

## Safety Selectors

Safety selectors are the controller's proof that the rendered identity stays inside the intended tenant boundary.

Every rendered identity must include:

- the binding namespace selector
- the workload service account selector
- selectors derived from the referenced pool
- the container discriminator when the mode is `PerObjective`

`kleym-operator` refuses to reconcile when it cannot prove those constraints.

## See Also

- [Architecture](/architecture/)
- [Basic Binding](/examples/basic-binding/)
- [Operator Spec](/spec/operator/)
