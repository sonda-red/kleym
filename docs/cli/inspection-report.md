---
title: Inspection Report
weight: 30
aliases:
  - /reference/inspection-report/
---

`BindingInspectionReport` is the canonical single-binding inspection result.
`KleymStatusReport` is the canonical cluster overview result. JSON is the
stable machine contract. Text is the human-oriented view over the same report
data.

Generate the canonical report with:

```sh
kleym inspect binding <name> -n <namespace> -o json
```

Generate the canonical status report with:

```sh
kleym status -o json
```

## Status Shape

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "KleymStatusReport",
  "generatedAt": "",
  "status": "",
  "cliVersion": "",
  "components": {},
  "config": {},
  "summary": {},
  "findings": []
}
```

## Inspection Shape

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "BindingInspectionReport",
  "generatedAt": "",
  "identityConfig": {},
  "bindingRef": {},
  "resolvedInput": {},
  "renderedIdentity": {},
  "renderedClusterSPIFFEID": {},
  "matchedPods": [],
  "findings": []
}
```

## Inspection Fields

| Field | Meaning |
| --- | --- |
| `identityConfig` | Trust domain and `ClusterSPIFFEID` class name used to render output, plus per-field source (`flag`, `bindingStatus`, or `default`). |
| `bindingRef` | Binding identity, generation, mode, refs, and current conditions. |
| `resolvedInput` | Resolved GAIE inputs, served GVKs, selector provenance, and container name. |
| `renderedIdentity` | SPIFFE ID and selectors rendered from the binding and resolved inputs. |
| `renderedClusterSPIFFEID` | Deterministic managed `ClusterSPIFFEID` name and rendered spec fields. |
| `matchedPods` | Readable pods or containers that match rendered Kubernetes-observable selectors. |
| `findings` | Typed inspection findings. |

## Status Fields

| Field | Meaning |
| --- | --- |
| `status` | Aggregate status: `OK`, `WARNING`, or `ERROR`. |
| `cliVersion` | Linked CLI version. |
| `components` | Overall Kleym health, operator deployment details, Kleym API versions, SPIRE CRD versions, and Gateway API Inference Extension CRD versions. |
| `config` | Visible trust domain and `ClusterSPIFFEID` class configuration. |
| `summary.bindings` | Counts of bindings by `ok`, `warning`, `error`, total, and true condition counts. |
| `findings` | Typed status findings. |

Matched pods are not proof that SPIRE issued an SVID, that a workload was
attested, or that an application consumed an identity.

See [Results](/cli/results/) for output-format guidance and [Findings](/cli/findings/) for
the current finding classes.
