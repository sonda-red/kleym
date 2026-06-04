---
title: Inspection Report
weight: 30
aliases:
  - /reference/inspection-report/
---

`BindingInspectionReport` is the canonical inspection result. JSON is the stable
machine contract. Text is the human-oriented view over the same report data.

Generate the canonical report with:

```sh
kleym inspect binding <name> -n <namespace> -o json
```

## Top-Level Shape

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "BindingInspectionReport",
  "generatedAt": "",
  "identityConfig": {},
  "bindingRef": {},
  "resolvedInput": {},
  "desired": {},
  "observed": {},
  "findings": [],
  "capabilities": {}
}
```

## Core Fields

| Field | Meaning |
| --- | --- |
| `identityConfig` | Trust domain and `ClusterSPIFFEID` class name used to render desired output, plus per-field source (`flag`, `bindingStatus`, or `default`). |
| `bindingRef` | Binding identity, generation, mode, refs, and current conditions. |
| `resolvedInput` | Resolved GAIE inputs, served GVKs, selector provenance, and container name. |
| `desired` | Desired `ClusterSPIFFEID` name, SPIFFE ID, class name, selectors, hint, and fallback value. |
| `observed` | Managed `ClusterSPIFFEID` resources, status, drift, and eligible workloads when pod reads are available. |
| `findings` | Typed inspection findings. |
| `capabilities` | Completeness for each inspection area. |

Workload eligibility means a pod or container matches rendered selectors. It is
not proof that an application fetched or used an SVID.

Capability states are `full`, `partial`, `skipped`, or `unknown`. If RBAC or
missing CRDs prevent a non-fatal check, report limited capability instead of
guessing.

See [Results](/cli/results/) for output-format guidance and [Findings](/cli/findings/) for
the current finding classes.
