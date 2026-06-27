---
title: GAIE Compatibility
weight: 50
description: "Gateway API Inference Extension compatibility matrix for supported InferencePool and InferenceObjective API groups and versions."
aliases:
  - /operator/reference/gaie-compatibility/
---

## Supported Inputs

| Object | Supported GVKs | Consumed fields |
| --- | --- | --- |
| `InferencePool` | `inference.networking.k8s.io/v1`; `inference.networking.x-k8s.io/v1alpha2` | `spec.selector.matchLabels`; flat string label maps are normalized for compatibility |
| `InferenceObjective` | `inference.networking.x-k8s.io/v1alpha2`; `inference.networking.k8s.io/v1` when served | `spec.poolRef`; only required for `PerObjective` or when `objectiveRef` is set |

GVK examples:

- `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective`
- `inference.networking.k8s.io/v1, Kind=InferencePool`

`InferenceModel` is legacy and is not a `kleym-operator` identity input.

## Selector Compatibility

`kleym-operator` accepts deterministic `InferencePool` selectors from
`spec.selector.matchLabels` or a flat selector map that can be normalized into
`matchLabels`.

`kleym-operator` rejects empty selectors, invalid label keys or values, values with leading
or trailing whitespace, non-empty `matchExpressions`, and selector shapes that
cannot be decoded into a stable label map.

## Discovery Behavior

At startup, `kleym-operator` discovers supported GAIE GVKs and watches only the
served subset. Startup fails when no supported pool GVK is served. Objective
GVKs are optional for `PoolOnly`; `PerObjective` waits for objective CRDs.

If `poolRef.group`, `objectiveRef.group`, or objective `spec.poolRef.group` is
set, it must match a supported GAIE group.
