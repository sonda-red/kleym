---
title: Findings
weight: 40
description: "Reference for kleym CLI finding severities, warning and error cases, and how findings map to inspection behavior."
aliases:
  - /reference/findings/
---

Findings are typed report entries. They are part of the stable JSON machine
contract.

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
| `operator-unavailable` | `error` | The Kleym operator Deployment is not discoverable or has no ready replicas. |
| `crd-missing` | `error` | A CRD required for status evaluation is not installed. |
| `binding-unhealthy` | `error` | A visible binding reports `Ready=False`. |
| `invalid-ref` | `error` | The referenced pool is missing or invalid. |
| `dependency-missing` | `error` when required | Required inputs or APIs are unavailable. |
| `unsafe-selector` | `error` | Rendered selectors fail Kleym safety rules. |
| `render-failure` | `error` | Identity output could not be rendered. |
| `zero-matched-pods` | `info` | Scale-to-zero can be valid. |
| `identity-config-undiscovered` | `warning` | Binding status does not include operator config, so inspection used CLI flags, defaults, or both. |
| `rbac-limited` | `warning` | Inspection continued with reduced visibility. |
| `unsupported-selector` | `warning` | Pod inspection cannot fully evaluate a rendered selector type. |
