---
title: Concepts
weight: 10
description: "Kleym concepts for identity-boundary-scoped inference variants, Gateway API Inference Extension InferencePool inputs, structural exclusivity, and selector safety."
aliases:
  - /operator/concepts/
---

## Inference Workload Identity

Kleym uses inference workload identity to mean identity registration derived from a Kubernetes inference serving boundary, not from a pod alone. In the current contract, that serving boundary is a Gateway API Inference Extension (GAIE) `InferencePool` referenced by an `InferenceIdentityBinding`.

The binding namespace and required service account constrain which workloads may match. Selectors from the referenced pool provide workload provenance, while the required `spec.identityBoundary` selects one label-defined workload variant. `kleym-operator` resolves the pool, validates structural exclusivity against peer bindings, and reconciles a managed SPIRE Controller Manager `ClusterSPIFFEID` only when the variant is exclusive. SPIRE Controller Manager translates that resource into registration entries; SPIRE Server issues SVIDs, while SPIRE Agent attests workloads and delivers credentials.

This stops at identity registration. Kleym does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or prove runtime SVID use. The authoritative behavior is defined in the [Operator Spec](/spec/operator/).

## GAIE Resources

- [`InferencePool` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool intent. `kleym-operator` resolves the pool named by `spec.poolRef.name` and derives selector input from `spec.selector`.
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical reference for GAIE resource schemas and status fields.

`kleym-operator` supports the documented `InferencePool` GVK in [GAIE Compatibility](/reference/gaie-compatibility/).

## Resolved Inference Target Identity

Kleym renders one identity for the required service account and resolved
inference target. The SPIFFE ID form is:

```text
spiffe://<trustDomain>/ns/<namespace>/sa/<serviceAccountName>/inference/pool/<pool-name>/variant/<labelValue>
```

The current GAIE `InferencePool` source resolves to anchor kind `pool` and an
anchor name equal to the pool name. The source GVK and binding name remain
provenance rather than identity path material. The same pool rendered for two
different service accounts therefore produces two distinct SPIFFE IDs. The
boundary label value identifies the variant within the pool.

## Safety Selectors

Safety selectors constrain the rendered workload match; they do not authorize
who may create bindings or assign workload labels.

Every rendered identity must include:

- the binding namespace selector
- the workload service account selector
- selectors derived from the referenced pool
- exactly one canonical `k8s:pod-label:<labelKey>:<labelValue>` boundary selector

`kleym-operator` refuses to reconcile when it cannot prove those constraints.
Within one namespace and service account, different SPIFFE IDs are structurally
exclusive only when they use the same boundary label key with different values.
Duplicate SPIFFE IDs, reused values, and different boundary keys fail closed.
The controller withdraws managed output for every member of a conflict group and
confirms it is absent before reporting the conflict as settled. A deleting peer
remains a competitor until its output is confirmed absent.

Structural exclusivity assumes that cluster admission restricts reserved
boundary labels to platform-controlled actors and keeps them immutable for each
Pod lifetime. See [Identity Boundary Admission Policy](/install/#identity-boundary-admission-policy).

## See Also

- [Architecture](/architecture/)
- [Basic Binding](/examples/basic-binding/)
- [Operator Spec](/spec/operator/)
