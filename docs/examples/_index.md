---
title: Examples
weight: 40
aliases:
  - /operator/examples/
---

Concrete manifests and expected reconciliation outcomes for common `InferenceIdentityBinding` flows.

Reusable reference inputs for e2e tests and demo docs live under
`test/reference/inference-environment/`. Those manifests represent externally
managed workload and GAIE resources that exist before `kleym-operator`
reconciles a binding.

## Before You Apply Examples

These examples assume:

- Gateway API Inference Extension (GAIE) CRDs are installed for [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/), and for [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) when using `PerObjective`
- SPIFFE Runtime Environment (SPIRE) Controller Manager is installed with the [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
- `kleym-operator` is running

The manifests show the GAIE fields `kleym-operator` consumes. For full GAIE
object shape and additional optional fields, use the [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/).
For SPIFFE and SPIRE background, see [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/) and [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/).

## Example Paths

| Example | Use it when | Outcome |
| --- | --- | --- |
| [Basic Binding](/examples/basic-binding/) | You need one identity per serving pool. | One managed `ClusterSPIFFEID` in `PoolOnly` mode. |
| [PerObjective](/examples/per-objective/) | Multiple objectives share a pool but need distinct identities. | One managed `ClusterSPIFFEID` per objective with a container-name selector. |

## Recommended Reading Order

1. Start with [Basic Binding](/examples/basic-binding/) to validate end-to-end wiring.
2. Move to [PerObjective](/examples/per-objective/) to apply model-level identity boundaries.
3. Review [Conditions](/reference/conditions/) and [Troubleshooting](/troubleshooting/) if reconciliation does not reach `Ready=True`.
