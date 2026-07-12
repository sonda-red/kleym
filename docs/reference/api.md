---
title: API
weight: 10
description: "InferenceIdentityBinding API reference covering poolRef, service accounts, required identity boundaries, conflict diagnosis, and rendered status."
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
| `serviceAccountName` | Yes | Admission-validated DNS-1123 subdomain Kubernetes service account required in every rendered identity selector set. |
| `identityBoundary.labelKey` | Yes | Valid Kubernetes label key under the reserved `identity.kleym.sonda.red/` prefix. |
| `identityBoundary.labelValue` | Yes | Valid, nonempty Kubernetes label value identifying the workload variant. |

Current validation rules enforced by the CRD:

- `poolRef.name` is required.
- `poolRef.group`, when set, must be `inference.networking.k8s.io`.
- `serviceAccountName` is required and admission-validated as a DNS-1123 subdomain with a maximum length of 253 characters.
- both identity-boundary fields are required; the key must use the reserved prefix and the value must be a nonempty Kubernetes label value.

## Status Fields

| Field | Meaning |
| --- | --- |
| `conditions` | Latest controller observations. |
| `trustDomain` | Operator trust domain used for the latest status update. |
| `clusterSPIFFEIDClassName` | Optional operator `ClusterSPIFFEID` class name used for the latest status update. Empty means classless output. |
| `identityBoundary` | Validated boundary label key and value retained for diagnosis. |
| `conflicts` | Deterministically sorted peer binding references, causes, SPIFFE IDs, and resolved peer boundary data. Present only for `Conflict=True`. |
| `computedSpiffeIDs` | Computed SPIFFE IDs produced from the pool binding. |
| `renderedSelectors` | Final selector set used for each rendered identity. |
| `pendingClusterSPIFFEID.name` | Deterministic managed-output name durably reserved before `Create`. |
| `pendingClusterSPIFFEID.claimID` | Controller-generated correlation token copied to the new object's `kleym.sonda.red/ownership-claim-id` annotation for safe ambiguous-create recovery. |
| `ownedClusterSPIFFEID.name` | Deterministic name of the confirmed managed-output incarnation. |
| `ownedClusterSPIFFEID.uid` | Kubernetes UID of the exact confirmed incarnation authorized for update or deletion. |
| `renderedClusterSPIFFEID.name` | Deterministic managed `ClusterSPIFFEID` name rendered for the binding. |
| `renderedClusterSPIFFEID.spiffeID` | Rendered SPIFFE ID written to the managed `ClusterSPIFFEID`. This matches the SPIFFE ID in `computedSpiffeIDs`. |
| `renderedClusterSPIFFEID.selectorFingerprint` | `sha256:<hex>` fingerprint of the canonical rendered selector set. |
| `renderedClusterSPIFFEID.observedGeneration` | Observed `metadata.generation` of the managed `ClusterSPIFFEID` when Kubernetes reports a persisted generation. Omitted when no persisted generation has been reported. |

On reference, selector, render, managed-output infrastructure, or managed-output
API failure, the operator clears `computedSpiffeIDs`, `renderedSelectors`, and
`renderedClusterSPIFFEID` together so status-only clients do not read stale
rendered output. Generic managed `ClusterSPIFFEID` read, create, update, or
delete API failures report `RenderFailure=True` with reason
`ManagedOutputApplyFailed`. Pending and confirmed ownership survive transient
API failures. NotFound clears a recorded incarnation; a missing or different
pending claim or confirmed UID marks a live same-name object foreign and leaves
it untouched.

## Kubectl Columns

`kubectl get inferenceidentitybindings.kleym.sonda.red` shows these CRD printer
columns for compact binding overviews:

| Column | Source |
| --- | --- |
| `POOL` | `spec.poolRef.name` |
| `BOUNDARY` | `status.identityBoundary.labelValue` |
| `READY` | `status.conditions[Ready].status` |
| `REASON` | `status.conditions[Ready].reason` |
| `SPIFFE ID` | `status.computedSpiffeIDs[0].spiffeID` |

Use `-A` to list bindings across namespaces. Status-derived columns are empty
until the operator has reconciled the binding and written status.

## Current Defaults

The controller always renders deterministic service-account-scoped inference
target SPIFFE IDs under its configured trust domain:

```text
spiffe://<trustDomain>/ns/<namespace>/sa/<serviceAccountName>/inference/pool/<pool-name>/variant/<labelValue>
```

The boundary label value is part of the identity path; the label key remains
selector and exclusivity-proof material.

## External Objects Resolved

The controller resolves `InferencePool` from the supported GAIE GVK. See [GAIE Compatibility](/reference/gaie-compatibility/) for the
consumed fields, group-constrained reference behavior, and startup discovery
rules.
