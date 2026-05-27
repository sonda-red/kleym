---
title: Dependencies
weight: 55
aliases:
  - /operator/reference/dependencies/
---

## Runtime Dependencies

| Dependency | Required for | Operator role |
| --- | --- | --- |
| Kubernetes API | Operator reconciliation. | Reads inputs and writes managed output. |
| `InferenceIdentityBinding` CRD | Binding API. | Kleym-owned CRD. |
| GAIE `InferencePool` CRD | Pool selector provenance. | Read-only input. |
| GAIE `InferenceObjective` CRD | `PerObjective` mode and bindings with `objectiveRef`. | Read-only input. |
| SPIRE Controller Manager `ClusterSPIFFEID` CRD | Managed registration output. | Written by `kleym-operator`. |
| SPIRE Controller Manager | SPIRE registration reconciliation. | External controller. |
| SPIRE Server and SPIRE Agent | SVID issuance and workload attestation. | External identity plane. |

`PoolOnly` bindings do not require the objective CRD. `kleym-operator` writes
`ClusterSPIFFEID` resources and does not write SPIRE entries directly.

## Compatibility Surfaces

| Surface | Source of truth |
| --- | --- |
| Go and Kubernetes library versions | `go.mod`, README, install docs |
| GAIE inputs | [GAIE Compatibility](/reference/gaie-compatibility/) |
| `ClusterSPIFFEID` output | [Managed Resources](/reference/resources/) |
| Condition behavior | [Conditions](/reference/conditions/) |
