---
title: Conditions
weight: 20
description: "Stable Kleym condition taxonomy and allowed reasons for the pool-only InferenceIdentityBinding operator contract."
aliases:
  - /operator/reference/conditions/
---

## Core Condition Contract

`InferenceIdentityBinding` uses only the following current pool-only condition types:

- `Ready`
- `InvalidRef`
- `UnsafeSelector`
- `RenderFailure`

These conditions describe operator reconciliation state only. They are not runtime evidence, SVID issuance, workload attestation, model loading, GPU usage, or request authorization signals. Future `InferenceSVIDConfig` behavior must define its own condition taxonomy instead of extending these binding reasons.

## Allowed Reasons

| Type | Meaning when `True` | Allowed `True` reasons |
| --- | --- | --- |
| `Ready` | The binding reconciled successfully. | `Reconciled` |
| `InvalidRef` | `poolRef` or the required GAIE pool CRD could not be resolved or validated. | `InvalidPoolRef`, `TargetPoolNotFound`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | The rendered selector set is missing required safety constraints or the pool selector cannot be rendered safely. | `InvalidPoolSelector`, `UnsafeSelector` |
| `RenderFailure` | Rendering or managed-output application failed after reference resolution succeeded. | `MissingTrustDomain`, `InvalidServiceAccountName`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing` |

`Ready=False` uses the same reason and message as the single active failure condition. Failure conditions use `Resolved` when `False`; all conditions may use `Initializing` while a generation has not been evaluated.

## Status Behavior

On successful reconciliation:

- `Ready=True` with reason `Reconciled`
- `InvalidRef=False` with reason `Resolved`
- `UnsafeSelector=False` with reason `Resolved`
- `RenderFailure=False` with reason `Resolved`

On any failure state:

- `Ready=False` with the primary failure reason and message
- Exactly one of `InvalidRef`, `UnsafeSelector`, or `RenderFailure` is set to `True` with the same reason and message
- The other non-triggering conditions are set to `False` with resolution or healthy messages
- `computedSpiffeIDs` and `renderedSelectors` are cleared

Dependency-unavailable states are classified as follows:

- Missing GAIE `InferencePool` CRD: `InvalidRef=True`, reason `InferencePoolCRDMissing`
- Missing SPIRE Controller Manager `ClusterSPIFFEID` CRD during reconcile: `RenderFailure=True`, reason `ClusterSPIFFEIDCRDMissing`

`ClusterSPIFFEIDCRDMissing` retries automatically on the controller's infrastructure retry timer. `InferencePoolCRDMissing` can appear during resolution, but the operator also fails startup if no supported GAIE pool GVK is served during controller setup.
