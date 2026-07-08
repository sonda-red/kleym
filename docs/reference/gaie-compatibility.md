---
title: GAIE Compatibility
weight: 50
description: "Gateway API Inference Extension compatibility matrix for the supported InferencePool API group and version."
aliases:
  - /operator/reference/gaie-compatibility/
---

## Supported Inputs

| Object | Supported GVK | Consumed fields |
| --- | --- | --- |
| `InferencePool` | `inference.networking.k8s.io/v1` | `spec.selector.matchLabels`; flat string label maps are normalized for compatibility |

GVK examples:

- `inference.networking.k8s.io/v1, Kind=InferencePool`

## Selector Compatibility

`kleym-operator` accepts deterministic `InferencePool` selectors from
`spec.selector.matchLabels` or a flat selector map that can be normalized into
`matchLabels`.

`kleym-operator` rejects empty selectors, invalid label keys or values, non-string
values, values with leading or trailing whitespace, any `matchExpressions` field,
and selector shapes that cannot be decoded into a stable label map.

## Discovery Behavior

At startup, `kleym-operator` discovers the supported GAIE `InferencePool` GVK and
watches it when served. Startup fails when the supported pool GVK is not served.

If `poolRef.group` is set, it must match a supported GAIE group.
