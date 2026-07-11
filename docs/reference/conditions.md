---
title: Conditions
weight: 20
description: "Stable InferenceIdentityBinding condition taxonomy for references, selector safety, identity-boundary conflicts, rendering, and managed output."
aliases:
  - /operator/reference/conditions/
---

## Core Condition Contract

`InferenceIdentityBinding` uses the following current pool-only condition types:

- `Ready`
- `InvalidRef`
- `UnsafeSelector`
- `Conflict`
- `RenderFailure`

These conditions describe reference resolution, selector safety, identity rendering, managed-output application, and reconciliation readiness.

## Allowed Reasons

| Type | Meaning when `True` | Allowed `True` reasons |
| --- | --- | --- |
| `Ready` | The binding reconciled successfully. | `Reconciled` |
| `InvalidRef` | `poolRef` or the required GAIE pool CRD could not be resolved or validated. | `InvalidPoolRef`, `TargetPoolNotFound`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | The rendered selector set or declared boundary cannot be rendered safely. | `InvalidPoolSelector`, `UnsafeSelector`, `InvalidIdentityBoundary` |
| `Conflict` | Structural exclusivity failed or the SPIFFE ID claim is duplicated. | `IdentityBoundaryConflict`, `DuplicateIdentityBinding` |
| `RenderFailure` | Rendering or managed-output application failed after reference resolution succeeded. | `MissingTrustDomain`, `InvalidServiceAccountName`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing`, `ManagedOutputApplyFailed` |

`Ready=False` uses the same reason and message as the single active failure condition. Failure conditions use `Resolved` when `False`; all conditions may use `Initializing` while a generation has not been evaluated.

## Status Behavior

On successful reconciliation:

- `Ready=True` with reason `Reconciled`
- `InvalidRef=False` with reason `Resolved`
- `UnsafeSelector=False` with reason `Resolved`
- `Conflict=False` with reason `Resolved`
- `RenderFailure=False` with reason `Resolved`

On any failure state:

- `Ready=False` with the primary failure reason and message
- Exactly one of `InvalidRef`, `UnsafeSelector`, `Conflict`, or `RenderFailure` is set to `True` with the same reason and message
- The other non-triggering conditions are set to `False` with resolution or healthy messages
- `computedSpiffeIDs`, `renderedSelectors`, and `renderedClusterSPIFFEID` are cleared
- pending or confirmed ownership remains present until the recorded output is confirmed absent

Conflict status is settled only after every managed output in the conflict set
has been confirmed absent. `status.conflicts` retains the precise peer diagnosis.

Dependency-unavailable states are classified as follows:

- Missing GAIE `InferencePool` CRD: `InvalidRef=True`, reason `InferencePoolCRDMissing`
- Missing SPIRE Controller Manager `ClusterSPIFFEID` CRD during reconcile: `RenderFailure=True`, reason `ClusterSPIFFEIDCRDMissing`
- Generic managed `ClusterSPIFFEID` list, create, update, or delete API failure: `RenderFailure=True`, reason `ManagedOutputApplyFailed`

`ClusterSPIFFEIDCRDMissing` retries automatically on the controller's infrastructure retry timer. `ManagedOutputApplyFailed` returns the API error so controller-runtime retries the failed reconcile. A NoMatch response from the managed-output API is not absence confirmation and therefore never clears ownership or permits finalizer removal. `InferencePoolCRDMissing` can appear during resolution, but the operator also fails startup if no supported GAIE pool GVK is served during controller setup.
