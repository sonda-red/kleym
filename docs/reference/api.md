---
title: API
weight: 10
description: "InferenceIdentityBinding API reference covering poolRef, service accounts, and status fields."
aliases:
  - /operator/reference/api/
---

## Primary Resource

- API group: `kleym.sonda.red`
- Version: `v1alpha1`
- Kind: `InferenceIdentityBinding`
- Scope: namespaced

`InferenceIdentityBinding` expresses identity intent for a single `InferencePool`. It drives reconciliation of managed `ClusterSPIFFEID` resources.

External Gateway API Inference Extension (GAIE) schema references:

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
| `poolRef.group` | No | Constrains pool resolution to `inference.networking.k8s.io`. |
| `serviceAccountName` | Yes | Kubernetes service account required in every rendered identity selector set. |

Current validation rules enforced by the CRD:

- `poolRef.name` is required.
- `poolRef.group`, when set, must be `inference.networking.k8s.io`.
- `serviceAccountName` is required.

## Status Fields

| Field | Meaning |
| --- | --- |
| `conditions` | Latest controller observations. |
| `trustDomain` | Operator trust domain used for the latest status update. |
| `clusterSPIFFEIDClassName` | Optional operator `ClusterSPIFFEID` class name used for the latest status update. Empty means classless output. |
| `computedSpiffeIDs` | Computed SPIFFE IDs produced from the pool binding. |
| `renderedSelectors` | Final selector set used for each rendered identity. |
| `renderedClusterSPIFFEID.name` | Deterministic managed `ClusterSPIFFEID` name rendered for the binding. |
| `renderedClusterSPIFFEID.spiffeID` | Rendered SPIFFE ID written to the managed `ClusterSPIFFEID`. This matches the SPIFFE ID in `computedSpiffeIDs`. |
| `renderedClusterSPIFFEID.selectorFingerprint` | `sha256:<hex>` fingerprint of the canonical rendered selector set. |
| `renderedClusterSPIFFEID.observedGeneration` | Observed `metadata.generation` of the managed `ClusterSPIFFEID` when the managed resource exists and can be read. Omitted when absent, unreadable, unavailable, or not persisted. |

On reference, selector, render, or managed-output infrastructure failure, the
operator clears `computedSpiffeIDs`, `renderedSelectors`, and
`renderedClusterSPIFFEID` together so status-only clients do not read stale
rendered output.

## Kubectl Columns

`kubectl get inferenceidentitybindings.kleym.sonda.red` shows these CRD printer
columns for compact binding overviews:

| Column | Source |
| --- | --- |
| `POOL` | `spec.poolRef.name` |
| `READY` | `status.conditions[Ready].status` |
| `REASON` | `status.conditions[Ready].reason` |
| `SPIFFE ID` | `status.computedSpiffeIDs[0].spiffeID` |

Use `-A` to list bindings across namespaces. Status-derived columns are empty
until the operator has reconciled the binding and written status.

## Current Defaults

The controller always renders deterministic pool SPIFFE IDs under its configured trust domain:

```text
spiffe://<trustDomain>/ns/<namespace>/pool/<pool-name>
```

## External Objects Resolved

The controller resolves `InferencePool` from the supported GAIE GVK. See [GAIE Compatibility](/reference/gaie-compatibility/) for the
consumed fields, group-constrained reference behavior, and startup discovery
rules.
