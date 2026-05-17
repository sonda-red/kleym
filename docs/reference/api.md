---
title: API
weight: 10
---

This page records the stable API facts exposed by the current scaffold. Behavioral contract still lives in the [Operator Spec](/spec/operator/).

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
| `spiffeIDTemplate` | No | Overrides the computed SPIFFE ID when provided. |
| `selectorSource` | Yes | Current enum: `DerivedFromPool`. |
| `workloadSelectorTemplates` | Yes | Non-empty set of user-supplied SPIRE workload selector templates. |
| `mode` | No | `PoolOnly` or `PerObjective`. Defaults to `PerObjective`. |
| `containerDiscriminator.type` | Conditionally | Required in `PerObjective`. Current enum: `ContainerName`, `ContainerImage`. |
| `containerDiscriminator.value` | Conditionally | Required in `PerObjective`. |

Current validation rules enforced by the CRD:

- `containerDiscriminator` must be empty when `mode` is `PoolOnly`.
- `containerDiscriminator` is required when `mode` is `PerObjective`, including the defaulted case.
- `objectiveRef` is required when `mode` is `PerObjective`, including the defaulted case.
- `workloadSelectorTemplates` must contain at least one entry.

## Status Fields

| Field | Meaning |
| --- | --- |
| `conditions` | Latest controller observations. |
| `computedSpiffeIDs` | Computed SPIFFE IDs with the mode that produced them. |
| `renderedSelectors` | Final selector set used for each rendered identity. |

## Current Defaults

When `spiffeIDTemplate` is omitted, the controller currently renders:

- `PoolOnly`: `spiffe://kleym.sonda.red/ns/<namespace>/pool/<pool-name>`
- `PerObjective`: `spiffe://kleym.sonda.red/ns/<namespace>/objective/<objective-name>`

When `mode` is omitted, the controller behaves as `PerObjective`.

## External Objects Resolved

The current controller resolves `InferenceObjective` and `InferencePool` from these candidate GVKs:

- `inference.networking.k8s.io/v1`
- `inference.networking.x-k8s.io/v1alpha2`

For pools, `kleym` currently reads `spec.selector`.

For objectives, `kleym` currently reads `spec.poolRef` only when `objectiveRef`
is set or required by `PerObjective`.

If `poolRef.group`, `objectiveRef.group`, or objective `spec.poolRef.group` is
set, it must match one of the supported GAIE groups listed above.
