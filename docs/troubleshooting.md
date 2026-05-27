---
title: Troubleshooting
weight: 60
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
| `InvalidRef` | `TargetObjectiveNotFound` | `spec.objectiveRef.name` does not resolve to an `InferenceObjective` in the same namespace. | Check the objective name, namespace, and installation of the objective CRD. |
| `InvalidRef` | `InvalidPoolRef` | The binding has an invalid or unsupported-group `poolRef`. | Fix `spec.poolRef` so it points to a valid pool in the same namespace and a supported GAIE group. |
| `InvalidRef` | `InvalidObjectiveRef` | The objective reference is invalid or its `spec.poolRef` does not point at the binding pool. | Fix `spec.objectiveRef` or the objective's `spec.poolRef` so both point at the same pool. |
| `InvalidRef` | `TargetPoolNotFound` | The binding points to an `InferencePool` that does not exist. | Create the pool or correct `spec.poolRef`. |
| `InvalidRef` | `InferenceObjectiveCRDMissing` | The GAIE `InferenceObjective` CRD is not installed and the binding needs an objective. | Install the objective CRD or use `PoolOnly` without `objectiveRef`. |
| `InvalidRef` | `InferencePoolCRDMissing` | The GAIE `InferencePool` CRD is not installed in the cluster. | Install the required GAIE CRDs before reconciling bindings. |
| `UnsafeSelector` | `InvalidPoolSelector` | The pool selector cannot be normalized into a rendered selector set, or its label keys or values are malformed. | Use a deterministic `matchLabels`-style selector with valid Kubernetes label keys and values. Do not rely on whitespace trimming. |
| `UnsafeSelector` | `UnsafeSelector` | The rendered selector set is missing namespace or service account safety constraints, or would widen beyond the tenant boundary. | Ensure the binding renders the required namespace and service account selectors and that the pool-derived selector is safe. |
| `Conflict` | `IdentityCollision` | Two `PerObjective` bindings resolve to the same workload slice, usually the same pool plus the same container discriminator. | Give each objective a distinct discriminator, change the pool mapping, or use `PoolOnly` if model-level separation is not required. |
| `RenderFailure` | `MissingObjectiveRef` | The effective mode is `PerObjective` but no objective subject was provided. | Add `objectiveRef` or switch to `PoolOnly`. |
| `RenderFailure` | `MissingContainerDiscriminator` | The effective mode is `PerObjective` but no discriminator was provided. | Add `containerDiscriminator` or switch to `PoolOnly`. |
| `RenderFailure` | `InvalidContainerDiscriminator` | The discriminator type or value is invalid. | Use a supported discriminator type with a non-empty value. |
| `RenderFailure` | `SelectorTemplateRenderFailed` | A workload selector template could not be rendered. | Fix the selector template values so they render to valid SPIRE workload selectors. |
| `RenderFailure` | `SPIFFEIDRenderFailed` | The SPIFFE ID template could not be rendered. | Fix the template or remove it to use the built-in default. |
| `RenderFailure` | `InvalidSPIFFEID` | The rendered SPIFFE ID is not valid. | Correct the template output so it is a valid SPIFFE ID. |
| `RenderFailure` | `UnsupportedMode` | The binding mode is not one of the supported values. | Use `PoolOnly` or `PerObjective`. |
| `RenderFailure` | `ClusterSPIFFEIDCRDMissing` | SPIRE Controller Manager or its `ClusterSPIFFEID` CRD is missing. | Install SPIRE Controller Manager and confirm the `clusterspiffeids.spire.spiffe.io` CRD exists. |

## Missing CRDs

`kleym-operator` depends on external CRDs as inputs and outputs. `InferencePool` is required for all bindings; `InferenceObjective` is only required for `PerObjective` or when `objectiveRef` is set.

`GVK` means `GroupVersionKind` (`<api-group>/<version>, Kind=<kind>`). In this context:

- `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective`
- `inference.networking.k8s.io/v1, Kind=InferenceObjective`
- `inference.networking.k8s.io/v1, Kind=InferencePool`
- `inference.networking.x-k8s.io/v1alpha2, Kind=InferencePool`

Check for:

```sh
kubectl get crd inferenceobjectives.inference.networking.k8s.io
kubectl get crd inferenceobjectives.inference.networking.x-k8s.io
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
Example: cluster has `InferencePool` only in `inference.networking.k8s.io/v1`, so startup logs skip messages for the other supported GAIE GVKs but can still reconcile `PoolOnly` bindings.

You can confirm what is actually served via:

```sh
kubectl api-resources --api-group=inference.networking.x-k8s.io
kubectl api-resources --api-group=inference.networking.k8s.io
```

Reference docs:

- [`InferenceObjective` API type](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/)
- [`InferencePool` API type](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
- [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)

When a CRD is missing, the reconciler keeps retrying automatically on a timer, so it can recover after installation without waiting for unrelated watch events.
If you install a newly supported GAIE CRD after `kleym-operator` has already started, restart the controller so startup-time GVK discovery can register the new watches.

## Collision Triage

If you hit `IdentityCollision`, compare all `PerObjective` bindings in the namespace that target the same pool and container discriminator.

Most collisions come from one of these situations:

- two bindings point at objectives backed by the same pool and the same serving container
- a copied manifest changed the objective name but kept the same discriminator
- a workload only has one serving container, so `PerObjective` cannot safely separate identities

Read [Collision Detection](/design/collision-detection/) if you need the exact controller rule.
