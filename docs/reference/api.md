---
title: API
weight: 10
---

This page records the stable API facts exposed by the current scaffold. Behavioral contract still lives in the [spec](../spec).

## Primary Resource

- API group: `kleym.sonda.red`
- Version: `v1alpha1`
- Kind: `InferenceIdentityBinding`
- Scope: namespaced

`InferenceIdentityBinding` expresses identity intent for a single `InferenceObjective` and drives reconciliation of managed `ClusterSPIFFEID` resources.

## Spec Fields

| Field | Required | Notes |
| --- | --- | --- |
| `targetRef.name` | Yes | References an `InferenceObjective` in the same namespace. |
| `spiffeIDTemplate` | No | Overrides the computed SPIFFE ID when provided. |
| `selectorSource` | Yes | Current enum: `DerivedFromPool`. |
| `workloadSelectorTemplates` | Yes | Non-empty set of rendered SPIRE workload selector strings. |
| `mode` | No | `PoolOnly` or `PerObjective`. Defaults to `PerObjective`. |
| `containerDiscriminator.type` | Conditionally | Required in `PerObjective`. Current enum: `ContainerName`, `ContainerImage`. |
| `containerDiscriminator.value` | Conditionally | Required in `PerObjective`. |

Current validation rules enforced by the CRD:

- `containerDiscriminator` must be empty when `mode` is `PoolOnly`.
- `containerDiscriminator` is required when `mode` is `PerObjective`, including the defaulted case.
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

For objectives, `kleym` currently reads `spec.poolRef`.

For pools, `kleym` currently reads `spec.selector`.
