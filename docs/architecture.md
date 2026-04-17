---
title: Architecture
weight: 20
---

## Control Flow

```text
InferenceIdentityBinding
        |
        | targetRef
        v
InferenceObjective
        |
        | spec.poolRef
        v
InferencePool
        |
        | selector + container discriminator + safety selectors
        v
      kleym
        |
        | rendered SPIFFE ID + rendered workload selectors
        v
ClusterSPIFFEID
        |
        v
SPIRE Controller Manager
        |
        v
SPIRE registration state
```

## Responsibility Boundaries

- `InferenceIdentityBinding` expresses identity intent.
- `InferenceObjective` tells `kleym` which serving pool an objective uses.
- `InferencePool` provides the workload provenance `kleym` turns into selector input.
- `kleym` validates the references, enforces selector safety, detects deterministic collisions, and renders `ClusterSPIFFEID`.
- SPIRE Controller Manager applies the `ClusterSPIFFEID` objects and manages SPIRE registration state.
- SPIRE Server and Agent remain responsible for issuance and rotation.

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
