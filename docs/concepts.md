---
title: Concepts
weight: 10
description: "Kleym concepts for Gateway API Inference Extension InferencePool inputs, pool identity, and selector safety."
aliases:
  - /operator/concepts/
---

This page describes Kleym-specific concepts. For the neutral category definition before the project mapping, read [Inference Workload Identity for Kubernetes](/concepts/inference-workload-identity/).

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

- [Inference Workload Identity for Kubernetes](/concepts/inference-workload-identity/)
- [Architecture](/architecture/)
- [Basic Binding](/examples/basic-binding/)
- [Operator Spec](/spec/operator/)
