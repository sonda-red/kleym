---
title: Selector Safety
weight: 20
description: "Selector safety design for proving namespace, service account, and pool boundaries before rendering SPIFFE identities."
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

If the namespace selector does not match the binding namespace, reconciliation fails.

If no service account selector is present, reconciliation fails.

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

Users provide only `serviceAccountName`. Kleym renders the namespace and
service-account selectors internally and derives pool selectors from `poolRef`.
The final selector set is de-duplicated and sorted before it is reported in
status or written to managed `ClusterSPIFFEID` output.
