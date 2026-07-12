---
title: Selector Safety
weight: 20
description: "Selector safety design for constraining namespace, service account, pool, and identity-boundary workload matches before rendering SPIFFE identities."
aliases:
  - /operator/design/selector-safety/
---

## Safety Goal

The controller writes identities for workloads that match the referenced pool,
binding namespace, and required service account selectors.

The authoritative rendered selector contract lives in
[Operator Spec](/spec/operator/#rendered-selector-contract). This design note
explains the safety rationale and must not broaden that contract.

## Current Safety Layers

The current implementation requires:

- a namespace selector: `k8s:ns:<binding-namespace>`
- a service account selector: `k8s:sa:<service-account>`
- selectors derived from the referenced pool
- one boundary selector rendered from required `spec.identityBoundary`:
  `k8s:pod-label:identity.kleym.sonda.red/variant:<variant>`

If the namespace selector does not match the binding namespace, reconciliation fails.

If no service account selector is present, reconciliation fails.

An invalid or missing variant fails with `UnsafeSelector=True` reason
`InvalidIdentityBoundary`. Different SPIFFE IDs in the same namespace and
service account are exclusive only when their variants differ. Other relationships conflict; the exact
pairwise rules remain authoritative in the
[Operator Spec](/spec/operator/#identity-boundary-and-exclusivity).

Conflict members retain no managed output. The controller withdraws their owned
`ClusterSPIFFEID` resources and confirms absence before reporting the conflict
as settled or permitting peers to recreate output.

## Pool Selector Constraints

The controller currently accepts only deterministic pool selectors:

- `spec.selector.matchLabels`
- or a flat selector map that can be normalized into `matchLabels`

The flat selector form is accepted because GAIE resources are read through `unstructured` objects and schema shape can vary across served versions. Normalizing either shape into `matchLabels` keeps selector rendering deterministic while preserving compatibility.

It rejects:

- empty selectors
- empty label keys or values
- non-string label values
- label keys or values that do not satisfy Kubernetes label syntax
- label values with leading or trailing whitespace
- any `matchExpressions` field
- pool selectors that cannot be decoded into a stable label map

The controller renders pool labels directly into SPIRE workload selectors, so it
rejects malformed label input instead of normalizing it into a selector the pool
did not specify. `matchExpressions` are not rendered.

## Selector Ownership

Users declare `serviceAccountName` and `identityBoundary`; Kleym renders the
namespace and service-account selectors internally and derives pool selectors
from `poolRef`. The final selector set is de-duplicated and sorted before it is
reported in status or written to managed `ClusterSPIFFEID` output.

Selector exclusivity depends on the boundary label remaining platform
controlled and immutable for the Pod lifetime. Kleym does not enforce workload
label writes; installation must provide the external admission policy described
in [Identity Boundary Admission Policy](/install/#identity-boundary-admission-policy).
