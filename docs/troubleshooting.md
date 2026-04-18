---
title: Troubleshooting
weight: 60
---

For the full condition set, read [reference/conditions](reference/conditions).

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
| `InvalidRef` | `TargetObjectiveNotFound` | `spec.targetRef.name` does not resolve to an `InferenceObjective` in the same namespace. | Check the objective name, namespace, and installation of the objective CRD. |
| `InvalidRef` | `InvalidPoolRef` | The referenced objective has an invalid or cross-namespace `poolRef`. | Fix the objective so `spec.poolRef` points to a valid pool in the same namespace. |
| `InvalidRef` | `TargetPoolNotFound` | The objective points to an `InferencePool` that does not exist. | Create the pool or correct the objective's `poolRef`. |
| `InvalidRef` | `InferenceObjectiveCRDMissing` | The GAIE `InferenceObjective` CRD is not installed in the cluster. | Install the required GAIE CRDs before reconciling bindings. |
| `InvalidRef` | `InferencePoolCRDMissing` | The GAIE `InferencePool` CRD is not installed in the cluster. | Install the required GAIE CRDs before reconciling bindings. |
| `UnsafeSelector` | `InvalidPoolSelector` | The pool selector cannot be normalized into the narrow selector shape `kleym` accepts. | Use a deterministic `matchLabels`-style selector and avoid unsupported selector forms. |
| `UnsafeSelector` | `UnsafeSelector` | The rendered selector set is missing namespace or service account safety constraints, or would widen beyond the tenant boundary. | Ensure the binding renders the required namespace and service account selectors and that the pool-derived selector is safe. |
| `Conflict` | `IdentityCollision` | Two `PerObjective` bindings resolve to the same workload slice, usually the same pool plus the same container discriminator. | Give each objective a distinct discriminator, change the pool mapping, or use `PoolOnly` if model-level separation is not required. |
| `RenderFailure` | `MissingContainerDiscriminator` | The effective mode is `PerObjective` but no discriminator was provided. | Add `containerDiscriminator` or switch to `PoolOnly`. |
| `RenderFailure` | `InvalidContainerDiscriminator` | The discriminator type or value is invalid. | Use a supported discriminator type with a non-empty value. |
| `RenderFailure` | `SelectorTemplateRenderFailed` | A workload selector template could not be rendered. | Fix the selector template values so they render to valid SPIRE workload selectors. |
| `RenderFailure` | `SPIFFEIDRenderFailed` | The SPIFFE ID template could not be rendered. | Fix the template or remove it to use the built-in default. |
| `RenderFailure` | `InvalidSPIFFEID` | The rendered SPIFFE ID is not valid. | Correct the template output so it is a valid SPIFFE ID. |
| `RenderFailure` | `UnsupportedMode` | The binding mode is not one of the supported values. | Use `PoolOnly` or `PerObjective`. |
| `RenderFailure` | `ClusterSPIFFEIDCRDMissing` | SPIRE Controller Manager or its `ClusterSPIFFEID` CRD is missing. | Install SPIRE Controller Manager and confirm the `clusterspiffeids.spire.spiffe.io` CRD exists. |

## Missing CRDs

`kleym` depends on external CRDs as inputs and outputs.

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

During startup, `kleym` discovers supported GAIE GVKs and logs a warning for each unavailable one:

```text
... skipping unavailable GVK ...
```

These warnings are expected when your cluster intentionally serves only part of the compatibility matrix.
Example: cluster has `InferenceObjective` only in `inference.networking.x-k8s.io/v1alpha2`, so startup logs a skip warning for `inference.networking.k8s.io/v1, Kind=InferenceObjective`.

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
If you install a newly supported GAIE CRD after `kleym` has already started, restart the controller so startup-time GVK discovery can register the new watches.

## Collision Triage

If you hit `IdentityCollision`, compare all `PerObjective` bindings in the namespace that target the same pool and container discriminator.

Most collisions come from one of these situations:

- two bindings point at objectives backed by the same pool and the same serving container
- a copied manifest changed the objective name but kept the same discriminator
- a workload only has one serving container, so `PerObjective` cannot safely separate identities

Read [collision detection](design/collision-detection) if you need the exact controller rule.
