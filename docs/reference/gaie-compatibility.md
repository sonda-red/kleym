---
title: GAIE Compatibility
weight: 50
description: "Gateway API Inference Extension compatibility matrix for supported InferencePool API groups and versions."
aliases:
  - /operator/reference/gaie-compatibility/
---

## Supported Inputs

| Object | Supported GVKs | Consumed fields |
| --- | --- | --- |
| `InferencePool` | `inference.networking.k8s.io/v1`; `inference.networking.x-k8s.io/v1alpha2` | `spec.selector.matchLabels`; flat string label maps are normalized for compatibility |

GVK examples:

- `inference.networking.k8s.io/v1, Kind=InferencePool`
- `inference.networking.x-k8s.io/v1alpha2, Kind=InferencePool`

`InferenceObjective` was removed upstream in
[kubernetes-sigs/gateway-api-inference-extension#2973](https://github.com/kubernetes-sigs/gateway-api-inference-extension/pull/2973)
and is not a current `kleym-operator` identity input. `InferenceModel` is
legacy and is also not an input.

## Selector Compatibility

`kleym-operator` accepts deterministic `InferencePool` selectors from
`spec.selector.matchLabels` or a flat selector map that can be normalized into
`matchLabels`.

`kleym-operator` rejects empty selectors, invalid label keys or values, values with leading
or trailing whitespace, non-empty `matchExpressions`, and selector shapes that
cannot be decoded into a stable label map.

## Discovery Behavior

At startup, `kleym-operator` discovers supported GAIE `InferencePool` GVKs and
watches only the served subset. Startup fails when no supported pool GVK is
served.

If `poolRef.group` is set, it must match a supported GAIE group.
