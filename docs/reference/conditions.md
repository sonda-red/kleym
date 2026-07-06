---
title: Conditions
weight: 20
description: "Kleym condition reference for Ready, InvalidRef, UnsafeSelector, RenderFailure, and related status reasons."
aliases:
  - /operator/reference/conditions/
---

## Condition Types

| Type | Meaning when `True` | Common reasons |
| --- | --- | --- |
| `Ready` | The binding reconciled successfully. | `Reconciled` |
| `InvalidRef` | `poolRef` or a required CRD could not be resolved or validated. | `TargetPoolNotFound`, `InvalidPoolRef`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | The rendered selector set is missing required safety constraints or the pool selector cannot be rendered safely. | `UnsafeSelector`, `InvalidPoolSelector` |
| `RenderFailure` | Rendering failed after reference resolution succeeded. | `InvalidServiceAccountName`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing`, `MissingTrustDomain` |

## Current Status Behavior

On successful reconciliation:

- `Ready=True` with reason `Reconciled`
- `InvalidRef=False`
- `UnsafeSelector=False`
- `RenderFailure=False`

On any failure state:

- `Ready=False`
- The triggering condition is set to `True`
- The other non-triggering conditions are set to `False` with resolution or healthy messages
- `computedSpiffeIDs` and `renderedSelectors` are cleared
