---
title: Concepts
weight: 10
description: "Kleym concepts for Gateway API Inference Extension inputs, PoolOnly and PerObjective identity modes, container-name boundaries, and selector safety."
aliases:
  - /operator/concepts/
---

This page describes Kleym-specific concepts. For the neutral category definition before the project mapping, read [Inference Workload Identity for Kubernetes](/concepts/inference-workload-identity/).

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

## Why Container Names Exist

`PerObjective` only makes sense if `kleym-operator` can prove that one objective identity lands on one workload slice.

When several objectives share the same `InferencePool`, the pod selector alone is not enough because every objective may resolve to the same pods. `containerName` adds a `k8s:container-name:<containerName>` selector so the identity applies to the intended serving container instead of the whole pod.

Without this extra boundary, distinct objective identities could collapse onto the same workload selection.

## Safety Selectors

Safety selectors are the controller's proof that the rendered identity stays inside the intended tenant boundary.

Every rendered identity must include:

- the binding namespace selector
- the workload service account selector
- selectors derived from the referenced pool
- the container-name selector when the mode is `PerObjective`

`kleym-operator` refuses to reconcile when it cannot prove those constraints.

## See Also

- [Inference Workload Identity for Kubernetes](/concepts/inference-workload-identity/)
- [Architecture](/architecture/)
- [Basic Binding](/examples/basic-binding/)
- [Operator Spec](/spec/operator/)
