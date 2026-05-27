---
title: Findings
weight: 40
aliases:
  - /reference/findings/
---

Findings are typed report entries. They are part of the stable JSON and YAML
machine contract.

## Shape

```json
{
  "id": "",
  "severity": "info|warning|error",
  "reason": "",
  "message": "",
  "affectedRefs": []
}
```

Condition-derived findings should preserve existing Kleym condition types and
reasons where possible. See [Conditions](/reference/conditions/).

## Required Classes

| ID | Default severity | Notes |
| --- | --- | --- |
| `binding-not-found` | `error` | Requested binding is absent after a successful API lookup. |
| `invalid-ref` | `error` | A referenced pool or objective is missing or invalid. |
| `dependency-missing` | `error` when required | Optional missing checks belong in `capabilities`. |
| `unsafe-selector` | `error` | Rendered selectors fail Kleym safety rules. |
| `render-failure` | `error` | Desired state could not be rendered. |
| `kleym-collision` | `error` | Kleym detected a deterministic identity collision. |
| `zero-eligible-workloads` | `info` | Scale-to-zero can be valid. |
| `ambiguous-container-match` | `warning` | A `PerObjective` discriminator matches more than one container. |
| `observed-drift` | `warning` or `error` | Managed output differs from desired output. |
| `rbac-limited` | `warning` | Inspection continued with reduced visibility. |
| `unsupported-selector` | `warning` | Pod inspection cannot fully evaluate a rendered selector type. |
