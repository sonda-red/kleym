---
title: Concepts
weight: 10
description: "Kleym concepts for Gateway API Inference Extension InferencePool inputs, pool identity, and selector safety."
aliases:
  - /operator/concepts/
---

## GAIE Resources

- [`InferencePool` (GAIE API type)](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool intent. `kleym-operator` resolves the pool named by `spec.poolRef.name` and derives selector input from `spec.selector`.
- [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical reference for GAIE resource schemas and status fields.

`kleym-operator` supports `InferencePool` objects from both `inference.networking.k8s.io/v1` and `inference.networking.x-k8s.io/v1alpha2`. See [GAIE Compatibility](/reference/gaie-compatibility/) for the current supported GVK list.

`InferenceObjective` was removed upstream from the current GAIE API and is historical context for older Kleym designs. It is not a current Kleym identity source.

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
