---
title: Managed Resources
weight: 30
---

This page records the Kubernetes resources `kleym` writes and the objects it depends on to do that work.

External references:

- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)

## Primary Managed Output

`kleym` manages [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) resources in `spire.spiffe.io`.

Each managed object currently includes:

- `spec.spiffeIDTemplate`: the fully rendered SPIFFE ID
- `spec.podSelector`: the selector derived from the referenced pool
- `spec.workloadSelectorTemplates`: rendered safety selectors, pool-derived selectors, and the optional per-objective container selector

Managed `ClusterSPIFFEID` objects are labeled with:

- `kleym.sonda.red/managed-by=kleym`
- `kleym.sonda.red/binding-name=<binding-name>`
- `kleym.sonda.red/binding-namespace=<binding-namespace>`

The controller also uses the finalizer `kleym.sonda.red/inferenceidentitybinding-finalizer` to clean up managed `ClusterSPIFFEID` objects on deletion.

## Naming

Managed `ClusterSPIFFEID` names are deterministic and derived from:

- the `kleym` controller name
- binding namespace
- binding name
- rendered mode (`pool` or `objective`)
- a short hash of the SPIFFE ID

That keeps names DNS-safe while allowing the SPIFFE ID to remain the real identity contract.

## Other Resources Touched

| Resource | Role |
| --- | --- |
| `InferenceIdentityBinding` | Primary namespaced API owned by `kleym`. |
| [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) | Target object resolved from `spec.targetRef.name`. |
| [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) | Selector source resolved from the objective's `spec.poolRef`. |
| [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) | Managed output resource written by the reconciler. |

## Read And Watch Behavior

The controller:

- watches `InferenceIdentityBinding`
- watches supported `InferenceObjective` objects and maps them back to matching bindings
- watches supported `InferencePool` objects and maps them back to bindings whose objectives reference those pools
