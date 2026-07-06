---
title: Troubleshooting
weight: 60
description: "Troubleshooting guide for Kleym binding conditions, missing dependency CRDs, unsafe selectors, and inspection output."
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

If reconciliation fails, `Ready=False` and the triggering condition becomes `True`.

## Condition And Reason Map

| Condition | Reason | Typical cause | What to fix |
| --- | --- | --- | --- |
| `InvalidRef` | `InvalidPoolRef` | The binding has an invalid or unsupported-group `poolRef`. | Fix `spec.poolRef` so it points to a valid pool in the same namespace and a supported GAIE group. |
| `InvalidRef` | `TargetPoolNotFound` | The binding points to an `InferencePool` that does not exist. | Create the pool or correct `spec.poolRef`. |
| `InvalidRef` | `InferencePoolCRDMissing` | The GAIE `InferencePool` CRD is not installed in the cluster. | Install the required GAIE CRDs before reconciling bindings. |
| `UnsafeSelector` | `InvalidPoolSelector` | The pool selector cannot be normalized into a rendered selector set, or its label keys or values are malformed. | Use a deterministic `matchLabels`-style selector with valid Kubernetes label keys and values. Do not rely on whitespace trimming. |
| `UnsafeSelector` | `UnsafeSelector` | The rendered selector set is missing namespace or service account safety constraints, or would widen beyond the tenant boundary. | Ensure `serviceAccountName` and the pool-derived selector stay within the intended workload boundary. |
| `Conflict` | `IdentityCollision` | Historical Objective-era collision state. Current pool-only reconciliation should leave `Conflict=False`. | Confirm you are running the current CRD and controller. |
| `RenderFailure` | `InvalidServiceAccountName` | `spec.serviceAccountName` is empty or not a valid Kubernetes service account name. | Set `serviceAccountName` to the exact workload service account. |
| `RenderFailure` | `InvalidSPIFFEID` | The computed SPIFFE ID is not valid. | Check the referenced namespace and pool names. |
| `RenderFailure` | `MissingTrustDomain` | The operator has no trust domain configured. | Configure `--trust-domain` or `KLEYM_TRUST_DOMAIN` before starting the operator. |
| `RenderFailure` | `ClusterSPIFFEIDCRDMissing` | SPIRE Controller Manager or its `ClusterSPIFFEID` CRD is missing. | Install SPIRE Controller Manager and confirm the `clusterspiffeids.spire.spiffe.io` CRD exists. |

## Missing CRDs

`kleym-operator` depends on external CRDs as inputs and outputs. `InferencePool` is required for all bindings.

`GVK` means `GroupVersionKind` (`<api-group>/<version>, Kind=<kind>`). In this context:

- `inference.networking.k8s.io/v1, Kind=InferencePool`
- `inference.networking.x-k8s.io/v1alpha2, Kind=InferencePool`

Check for:

```sh
kubectl get crd inferencepools.inference.networking.k8s.io
kubectl get crd inferencepools.inference.networking.x-k8s.io
kubectl get crd clusterspiffeids.spire.spiffe.io
```

If your cluster uses the alternate GAIE API group version supported by the controller, confirm those CRDs are installed instead.

During startup, `kleym-operator` discovers supported GAIE GVKs and logs a warning for each unavailable one:

```text
... skipping unavailable GVK ...
```

These warnings are expected when your cluster serves only part of the
compatibility matrix.
Example: cluster has `InferencePool` only in `inference.networking.k8s.io/v1`, so startup logs skip messages for the other supported GAIE GVKs but can still reconcile bindings.

You can confirm what is actually served via:

```sh
kubectl api-resources --api-group=inference.networking.x-k8s.io
kubectl api-resources --api-group=inference.networking.k8s.io
```

Reference docs:

- [`InferencePool` API type](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)

When a CRD is missing, the reconciler keeps retrying automatically on a timer, so it can recover after installation without waiting for unrelated watch events.
If you install a newly supported GAIE CRD after `kleym-operator` has already started, restart the controller so startup-time GVK discovery can register the new watches.
