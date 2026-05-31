---
title: API
weight: 10
aliases:
  - /operator/reference/api/
---

## Primary Resource

- API group: `kleym.sonda.red`
- Version: `v1alpha1`
- Kind: `InferenceIdentityBinding`
- Scope: namespaced

`InferenceIdentityBinding` expresses identity intent for a single `InferencePool` and, in `PerObjective` mode, an `InferenceObjective` subject. It drives reconciliation of managed `ClusterSPIFFEID` resources.

External Gateway API Inference Extension (GAIE) schema references:

- [`InferenceObjective` (GAIE API)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/)
- [`InferencePool` (GAIE API)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/)

External SPIFFE/SPIRE references:

- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)

## Spec Fields

| Field | Required | Notes |
| --- | --- | --- |
| `poolRef.name` | Yes | References an `InferencePool` in the same namespace. |
| `poolRef.group` | No | Constrains pool resolution to a supported GAIE InferencePool group. |
| `objectiveRef.name` | Conditionally | Required in `PerObjective`; references an `InferenceObjective` in the same namespace. |
| `objectiveRef.group` | No | Constrains objective resolution to a supported GAIE InferenceObjective group. |
| `serviceAccountName` | Yes | Kubernetes service account required in every rendered identity selector set. |
| `mode` | No | `PoolOnly` or `PerObjective`. Defaults to `PerObjective`. |
| `containerName` | Conditionally | Required in `PerObjective`; forbidden in `PoolOnly`. |

Current validation rules enforced by the CRD:

- `containerName` must be empty when `mode` is `PoolOnly`.
- `containerName` is required when `mode` is `PerObjective`, including the defaulted case.
- `objectiveRef` is required when `mode` is `PerObjective`, including the defaulted case.
- `serviceAccountName` is required.

## Status Fields

| Field | Meaning |
| --- | --- |
| `conditions` | Latest controller observations. |
| `computedSpiffeIDs` | Computed SPIFFE IDs with the mode that produced them. |
| `renderedSelectors` | Final selector set used for each rendered identity. |

## Current Defaults

The controller always renders deterministic SPIFFE IDs under its configured trust domain:

- `PoolOnly`: `spiffe://<trustDomain>/ns/<namespace>/pool/<pool-name>`
- `PerObjective`: `spiffe://<trustDomain>/ns/<namespace>/objective/<objective-name>`

When `mode` is omitted, the controller behaves as `PerObjective`.

## External Objects Resolved

The controller resolves `InferencePool` and, when needed, `InferenceObjective`
from supported GAIE GVKs. See [GAIE Compatibility](/reference/gaie-compatibility/) for the
compatibility matrix, consumed fields, group-constrained reference behavior, and
startup discovery rules.
