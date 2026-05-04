---
title: Conditions
weight: 20
---

This page records the current status condition set exposed by `InferenceIdentityBinding`. The [spec](../spec) remains authoritative for intended behavior.

## Condition Types

| Type | Meaning when `True` | Common reasons |
| --- | --- | --- |
| `Ready` | The binding reconciled successfully. | `Reconciled` |
| `Conflict` | A per-objective collision blocks reconciliation. | `IdentityCollision` |
| `InvalidRef` | `targetRef`, `poolRef`, or a required CRD could not be resolved. | `TargetObjectiveNotFound`, `TargetPoolNotFound`, `InvalidPoolRef`, `UnsupportedPoolGroup`, `InferenceObjectiveCRDMissing`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | The rendered selector set is missing required safety constraints or the pool selector cannot be rendered safely. | `UnsafeSelector`, `InvalidPoolSelector` |
| `RenderFailure` | Rendering failed after reference resolution succeeded. | `SelectorTemplateRenderFailed`, `MissingContainerDiscriminator`, `InvalidContainerDiscriminator`, `SPIFFEIDRenderFailed`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing`, `UnsupportedMode` |

## Current Status Behavior

On successful reconciliation:

- `Ready=True` with reason `Reconciled`
- `Conflict=False`
- `InvalidRef=False`
- `UnsafeSelector=False`
- `RenderFailure=False`

On any failure state:

- `Ready=False`
- The triggering condition is set to `True`
- The other non-triggering conditions are set to `False` with resolution or healthy messages
- `computedSpiffeIDs` and `renderedSelectors` are cleared

On a detected per-objective collision:

- `Conflict=True`
- `Ready=False`
- Managed `ClusterSPIFFEID` resources for the colliding bindings are deleted until the collision is resolved

## Collision Scope

`Conflict` is only used for `PerObjective` bindings. `PoolOnly` bindings do not participate in the current collision-detection path.
