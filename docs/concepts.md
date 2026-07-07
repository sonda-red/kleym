---
title: Concepts
weight: 10
description: "Kleym concepts for inference workload identity, Gateway API Inference Extension InferencePool inputs, pool identity, and selector safety."
aliases:
  - /operator/concepts/
---

## Inference Workload Identity

Kleym uses inference workload identity to mean identity registration derived from a Kubernetes inference serving boundary, not from a pod alone. In the current contract, that serving boundary is a Gateway API Inference Extension (GAIE) `InferencePool` referenced by an `InferenceIdentityBinding`.

The binding namespace and required service account constrain which workloads may match. Selectors from the referenced pool provide workload provenance. `kleym-operator` combines those facts to render one deterministic pool SPIFFE ID and reconcile a managed SPIRE Controller Manager `ClusterSPIFFEID`.

This stops at identity registration. Kleym does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or prove runtime SVID use. The authoritative behavior is defined in the [Operator Spec](/spec/operator/).

## GAIE Resources

- [`InferencePool` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool intent. `kleym-operator` resolves the pool named by `spec.poolRef.name` and derives selector input from `spec.selector`.
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical reference for GAIE resource schemas and status fields.

`kleym-operator` supports the documented `InferencePool` GVK in [GAIE Compatibility](/reference/gaie-compatibility/).

## Pool Identity

Kleym renders one identity for the referenced serving pool. The SPIFFE ID form is:

```text
spiffe://<trustDomain>/ns/<namespace>/pool/<pool-name>
```

## Safety Selectors

Safety selectors are the controller's proof that the rendered identity stays inside the intended tenant boundary.

Every rendered identity must include:

- the binding namespace selector
- the workload service account selector
- selectors derived from the referenced pool

`kleym-operator` refuses to reconcile when it cannot prove those constraints.

## See Also

- [Architecture](/architecture/)
- [Basic Binding](/examples/basic-binding/)
- [Operator Spec](/spec/operator/)
