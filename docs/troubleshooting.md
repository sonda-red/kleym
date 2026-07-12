---
title: Troubleshooting
weight: 60
description: "Troubleshooting guide for Kleym binding condition reasons, missing dependency CRDs, unsafe selectors, and inspection output."
aliases:
  - /operator/troubleshooting/
---

For the full condition set, read [Conditions](/reference/conditions/).

## Start Here

Inspect the binding status and recent events:

```sh
kubectl get inferenceidentitybinding -n <namespace> <name> -o yaml
kubectl describe inferenceidentitybinding -n <namespace> <name>
```

In normal operation:

- `Ready=True` with reason `Reconciled`
- all failure conditions are `False` with reason `Resolved`

A settled validation, conflict, or managed-output failure sets `Ready=False`;
the single active failure condition carries the same reason and message.
`Ready=Unknown` with reason `Initializing` means reconciliation has not settled,
including while the controller waits to confirm old or conflicting managed
output is absent.

## Condition And Reason Map

| Condition | Reason | Typical cause | What to fix |
| --- | --- | --- | --- |
| `InvalidRef` | `InvalidPoolRef` | The binding has an invalid or unsupported-group `poolRef`. | Fix `spec.poolRef` so it points to a valid pool in the same namespace and a supported GAIE group. |
| `InvalidRef` | `TargetPoolNotFound` | The binding points to an `InferencePool` that does not exist. | Create the pool or correct `spec.poolRef`. |
| `InvalidRef` | `InferencePoolCRDMissing` | The GAIE `InferencePool` CRD is not installed in the cluster. | Install the required GAIE CRDs before reconciling bindings. |
| `UnsafeSelector` | `InvalidPoolSelector` | The pool selector cannot be normalized into a rendered selector set, or its label keys or values are malformed. | Use a deterministic `matchLabels`-style selector with valid Kubernetes label keys and values. Do not rely on whitespace trimming. |
| `UnsafeSelector` | `UnsafeSelector` | The rendered selector set is missing namespace or service account constraints, or would widen beyond the required workload match. | Ensure `serviceAccountName` and the pool-derived selector preserve the intended workload match. |
| `UnsafeSelector` | `InvalidIdentityBoundary` | The required `identityBoundary.variant` is missing, empty, malformed, or whitespace-padded. | Set a valid, nonempty Kubernetes label value in `identityBoundary.variant`. |
| `Conflict` | `VariantConflict` | Peer claims in the same namespace and service account reuse a variant for different SPIFFE IDs. | Give the bindings distinct variants, then wait for all conflicting output to be confirmed absent. |
| `Conflict` | `DuplicateSPIFFEID` | This binding and at least one peer render the same SPIFFE ID. | Remove the duplicate claim or change its identity inputs; selector differences do not make duplicate SPIFFE IDs valid. |
| `RenderFailure` | `InvalidServiceAccountName` | `spec.serviceAccountName` is empty or not a valid Kubernetes service account name. | Set `serviceAccountName` to the exact workload service account. |
| `RenderFailure` | `InvalidSPIFFEID` | The computed SPIFFE ID is not valid. | Check the referenced namespace and pool names. |
| `RenderFailure` | `MissingTrustDomain` | The operator has no trust domain configured. | Configure `--trust-domain` or `KLEYM_TRUST_DOMAIN` before starting the operator. |
| `RenderFailure` | `ClusterSPIFFEIDCRDMissing` | SPIRE Controller Manager or its `ClusterSPIFFEID` CRD is missing. | Install SPIRE Controller Manager and confirm the `clusterspiffeids.spire.spiffe.io` CRD exists. |
| `RenderFailure` | `ManagedOutputApplyFailed` | The Kubernetes API rejected or failed a managed `ClusterSPIFFEID` list, create, update, or delete request. | Check API server availability, RBAC, admission errors, and SPIRE Controller Manager CRD health; the controller returns the API error for retry. |

## Conflict Diagnosis

When `Conflict=True`, inspect `status.conflicts`. Each item identifies a peer
binding and a precise cause:

| Cause | Meaning |
| --- | --- |
| `VariantReuse` | Different SPIFFE IDs in the same namespace and service account reuse the same variant. |
| `DuplicateSPIFFEID` | Both bindings render the same SPIFFE ID; this is a duplicate claim regardless of selectors or boundary declarations. |

Conflict output is fail closed. The controller withdraws every owned
`ClusterSPIFFEID` in the conflict group and reports the settled conflict only
after absence is confirmed. While deletion or a prior identity replacement is
pending, expect cleared rendered output and `Ready=Unknown` with reason
`Initializing`. A deleting peer remains a competitor until its output absence
is confirmed, so do not recreate output manually.

## Missing CRDs

`kleym-operator` depends on external CRDs as inputs and outputs. `InferencePool` is required for all bindings.

`GVK` means `GroupVersionKind` (`<api-group>/<version>, Kind=<kind>`). In this context:

- `inference.networking.k8s.io/v1, Kind=InferencePool`

Check for:

```sh
kubectl get crd inferencepools.inference.networking.k8s.io
kubectl get crd clusterspiffeids.spire.spiffe.io
```

During startup, `kleym-operator` discovers the supported GAIE GVK and fails
startup if `inference.networking.k8s.io/v1, Kind=InferencePool` is not served.

You can confirm what is actually served via:

```sh
kubectl api-resources --api-group=inference.networking.k8s.io
```

Reference docs:

- [`InferencePool` API type](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)

After startup succeeds, missing managed-output CRDs or infrastructure-not-ready states keep retrying automatically on a timer, so they can recover after installation without waiting for unrelated watch events.
If you install the GAIE CRD after `kleym-operator` startup failed, restart the controller so startup-time GVK discovery can register the watch.

If the `ClusterSPIFFEID` CRD is missing during reconcile, the binding reports `RenderFailure=True` with reason `ClusterSPIFFEIDCRDMissing` and retries automatically. If the `InferencePool` CRD is missing during pool resolution, the binding reports `InvalidRef=True` with reason `InferencePoolCRDMissing`; if it was missing during startup, restart the operator after installing the CRD.
