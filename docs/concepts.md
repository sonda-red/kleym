---
title: Concepts
weight: 10
description: "Kleym concepts for service-account-scoped inference target identity, Gateway API Inference Extension InferencePool inputs, and selector safety."
aliases:
  - /operator/concepts/
---

## Inference Workload Identity

Kleym uses inference workload identity to mean identity registration derived from a Kubernetes inference serving boundary, not from a pod alone. In the current contract, that serving boundary is a Gateway API Inference Extension (GAIE) `InferencePool` referenced by an `InferenceIdentityBinding`.

The binding namespace and required service account constrain which workloads may match. Selectors from the referenced pool provide workload provenance. `kleym-operator` resolves the pool to an inference target, combines that target with the service-account boundary, and reconciles a managed SPIRE Controller Manager `ClusterSPIFFEID`.

This stops at identity registration. Kleym does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or prove runtime SVID use. The authoritative behavior is defined in the [Operator Spec](/spec/operator/).

## GAIE Resources

- [`InferencePool` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool intent. `kleym-operator` resolves the pool named by `spec.poolRef.name` and derives selector input from `spec.selector`.
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical reference for GAIE resource schemas and status fields.

`kleym-operator` supports the documented `InferencePool` GVK in [GAIE Compatibility](/reference/gaie-compatibility/).

## Resolved Inference Target Identity

Kleym renders one identity for the required service account and resolved
inference target. The SPIFFE ID form is:

```text
spiffe://<trustDomain>/ns/<namespace>/sa/<serviceAccountName>/inference/<anchor-kind>/<anchor-name>
```

The current GAIE `InferencePool` source resolves to anchor kind `pool` and an
anchor name equal to the pool name. The source GVK and binding name remain
provenance rather than identity path material. The same pool rendered for two
different service accounts therefore produces two distinct SPIFFE IDs.

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
