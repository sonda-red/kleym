---
title: Examples
weight: 40
---

Concrete manifests and expected reconciliation outcomes for common `InferenceIdentityBinding` flows.

## Before You Apply Examples

These examples assume:

- Gateway API Inference Extension (GAIE) CRDs are installed for [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- SPIFFE Runtime Environment (SPIRE) Controller Manager is installed with the [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
- the `kleym` controller is running

The manifests here intentionally show the minimal GAIE fields `kleym` consumes. For full GAIE object shape and additional optional fields, use the [GAIE API types index](https://gateway-api-inference-extension.sigs.k8s.io/api-types/).
For SPIFFE and SPIRE background, see [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/) and [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/).

## Example Paths

| Example | Use it when | Outcome |
| --- | --- | --- |
| [Basic Binding](basic-binding) | You need one identity per serving pool. | One managed `ClusterSPIFFEID` in `PoolOnly` mode. |
| [PerObjective](per-objective) | Multiple objectives share a pool but need distinct identities. | One managed `ClusterSPIFFEID` per objective, narrowed by container discriminator. |

## Recommended Reading Order

1. Start with [Basic Binding](basic-binding) to validate end-to-end wiring.
2. Move to [PerObjective](per-objective) to apply model-level identity boundaries.
3. Review [reference/conditions](../reference/conditions) and [troubleshooting](../troubleshooting) if reconciliation does not reach `Ready=True`.
