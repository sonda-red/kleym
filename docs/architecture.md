---
title: Architecture
weight: 20
summary: End-to-end control flow from `InferenceIdentityBinding` to SPIFFE Runtime Environment (SPIRE) registration state.
description: How `kleym-operator` resolves Gateway API Inference Extension (GAIE) resources, enforces selector safety, and reconciles `ClusterSPIFFEID`.
aliases:
  - /operator/architecture/
---

## Control Flow

This flow uses Gateway API Inference Extension (GAIE) objects as upstream inputs.

```
                InferenceIdentityBinding
                         │
                     Deleted? ──yes──▶ Clean up ClusterSPIFFEIDs
                         │                  Remove finalizer
                         no
                         │
                   Ensure finalizer
                         │
    InferencePool ───────▶ Resolve poolRef → Pool
                         │
                  Derive selectors from pool
                  Validate safety selectors
                  Render SPIFFE ID
                         │
                    Reconcile
                    ClusterSPIFFEID
                         │
                    ClusterSPIFFEID
                         │
                    SPIRE Controller Manager
                         │
                    SPIRE registration entries
                         │
                    Patch status
                    emit events
```

## External Contracts

- [`InferencePool` API](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool selector source used by `kleym-operator`.
- [Gateway API Inference Extension (GAIE) API types](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical schema reference for GAIE resources.
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/): identity model and SPIFFE ID/SVID concepts.
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/): server/agent architecture and attestation model.
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager): Kubernetes reconciler that applies `ClusterSPIFFEID`.
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md): output resource shape reconciled by `kleym-operator`.

## See Also

- Read [Concepts](/concepts/) for the pool identity and selector model.
- Read [Managed Resources](/reference/resources/) for the concrete output object shape.
- Read [Reconciliation](/design/reconciliation/) for the controller flow in more detail.
