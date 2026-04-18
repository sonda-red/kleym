---
title: Architecture
weight: 20
summary: End-to-end control flow from `InferenceIdentityBinding` to SPIFFE Runtime Environment (SPIRE) registration state.
description: How `kleym` resolves Gateway API Inference Extension (GAIE) resources, enforces selector safety, and reconciles `ClusterSPIFFEID`.
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
    InferenceObjective ──▶ Resolve targetRef → Objective
    InferencePool ───────▶ Extract and resolve poolRef → Pool
                         │
                  Derive selectors from pool
                  Add container discriminator (PerObjective)
                  Validate safety selectors
                  Render SPIFFE ID
                         │
                    Collision?
                    ╱        ╲
                 yes          no
                  │            │
          Set Conflict     Reconcile
          Clean up         ClusterSPIFFEID
          ClusterSPIFFEIDs      │
                  │        ClusterSPIFFEID
                  │             │
                  │        SPIRE Controller Manager
                  │             │
                  │        SPIRE registration entries
                  │            │
                  ╰──── Patch status ────╯
                        emit events
```

## Responsibility Boundaries

- `InferenceIdentityBinding` expresses identity intent.
- `InferenceObjective` tells `kleym` which serving pool an objective uses.
- `InferencePool` provides the workload provenance `kleym` turns into selector input.
- `kleym` validates the references, enforces selector safety, detects deterministic collisions, and renders `ClusterSPIFFEID`.
- SPIRE Controller Manager applies the `ClusterSPIFFEID` objects and manages SPIRE registration state.
- SPIRE Server and Agent remain responsible for issuance and rotation.

## External Contracts

- [`InferenceObjective` API](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/): objective-level inference intent and `poolRef`.
- [`InferencePool` API](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/): serving pool selector source used by `kleym`.
- [Gateway API Inference Extension (GAIE) API types](https://gateway-api-inference-extension.sigs.k8s.io/api-types/): canonical schema reference for GAIE resources.
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/): identity model and SPIFFE ID/SVID concepts.
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/): server/agent architecture and attestation model.
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager): Kubernetes reconciler that applies `ClusterSPIFFEID`.
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md): output resource shape reconciled by `kleym`.

## Why The Flow Matters

The architecture keeps identity registration derived from existing inference objects instead of from ad hoc labels or manual SPIRE entry management.

That gives `kleym` three useful properties:

- identity stays tied to current GAIE intent
- registration remains deterministic and idempotent
- issuance and rotation stay delegated to SPIRE instead of being reimplemented in the controller

## See Also

- Read [concepts](concepts) for the mode and selector model.
- Read [managed resources](reference/resources) for the concrete output object shape.
- Read [reconciliation](design/reconciliation) for the controller flow in more detail.
