---
title: Managed Resources
weight: 30
description: "Managed ClusterSPIFFEID output reference for labels, owner references, SPIFFE IDs, selectors, class names, and reconciliation ownership."
aliases:
  - /operator/reference/resources/
---

## Primary Managed Output

`kleym-operator` manages
[`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
resources in `spire.spiffe.io`.

## Rendered Field Mapping

| Field | Rendered value |
| --- | --- |
| `spec.spiffeIDTemplate` | Fully rendered SPIFFE ID. |
| `spec.podSelector` | Validated selector derived from the referenced pool. |
| `spec.workloadSelectorTemplates` | Rendered namespace and service-account safety selectors, pool-derived selectors, and the optional per-objective container-name selector. |
| `spec.className` | Rendered only when `kleym-operator` is configured with `--clusterspiffeid-class-name`. When omitted, SPIRE Controller Manager must watch classless resources. |
| `spec.fallback` | `false` for all managed identities. |
| `spec.hint` | Originating binding reference in the form `<namespace>/<binding-name>`. |
| JWT-SVID-related fields | Not rendered today. Requires a user story and SPIRE Controller Manager/SPIRE version gate. |

Managed `ClusterSPIFFEID` objects are labeled with:

- `kleym.sonda.red/managed-by=kleym`
- `kleym.sonda.red/binding-name=<binding-name>`
- `kleym.sonda.red/binding-namespace=<binding-namespace>`

The controller also uses the finalizer
`kleym.sonda.red/inferenceidentitybinding-finalizer` to clean up managed
`ClusterSPIFFEID` objects on deletion.

## Naming

Managed `ClusterSPIFFEID` names are deterministic and derived from:

- the `kleym-operator` controller name
- binding namespace
- binding name
- rendered mode (`pool` or `objective`)
- a short hash of the SPIFFE ID

That keeps names DNS-safe while allowing the SPIFFE ID to remain the real identity contract.

## Other Resources Touched

| Resource | Role |
| --- | --- |
| `InferenceIdentityBinding` | Source resource for managed output. |
| [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) | Required selector source resolved from `spec.poolRef.name`. |
| [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) | Optional objective subject resolved from `spec.objectiveRef.name` and validated against `spec.poolRef`. |
| [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) | Managed output resource written by the reconciler. |

## Read And Watch Behavior

The controller:

- watches `InferenceIdentityBinding`
- watches supported `InferencePool` objects and maps them back to bindings whose `spec.poolRef.name` references those pools
- watches supported `InferenceObjective` objects and maps them back to bindings whose optional `spec.objectiveRef.name` references those objectives
