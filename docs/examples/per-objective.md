---
title: PerObjective
weight: 20
---

This example shows the current `PerObjective` path, including the container discriminator that keeps one objective identity tied to one container selection.

As in the other examples, the Gateway API Inference Extension (GAIE) snippets focus on the fields `kleym` currently consumes. Your cluster may require additional GAIE fields.
For full GAIE schema details, see [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/).
Reference docs: [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/), [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/), and [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md).

## Input

```yaml
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: pool-a
  namespace: default
spec:
  selector:
    matchLabels:
      app: model-server
---
apiVersion: inference.networking.k8s.io/v1
kind: InferenceObjective
metadata:
  name: objective-a
  namespace: default
spec:
  poolRef:
    name: pool-a
---
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: objective-a
  namespace: default
spec:
  targetRef:
    name: objective-a
  selectorSource: DerivedFromPool
  workloadSelectorTemplates:
    - k8s:ns:default
    - k8s:sa:inference-sa
  mode: PerObjective
  containerDiscriminator:
    type: ContainerName
    value: main
```

## Expected Outcome

The binding should reconcile to a managed `ClusterSPIFFEID` with:

- SPIFFE ID `spiffe://kleym.sonda.red/ns/default/objective/objective-a`
- the pool-derived pod selector for `app=model-server`
- workload selectors including:
  - `k8s:ns:default`
  - `k8s:sa:inference-sa`
  - `k8s:pod-label:app:model-server`
  - `k8s:container-name:main`

Relevant output shape:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  labels:
    kleym.sonda.red/managed-by: kleym
    kleym.sonda.red/binding-name: objective-a
    kleym.sonda.red/binding-namespace: default
spec:
  spiffeIDTemplate: spiffe://kleym.sonda.red/ns/default/objective/objective-a
  podSelector:
    matchLabels:
      app: model-server
  workloadSelectorTemplates:
    - k8s:container-name:main
    - k8s:ns:default
    - k8s:pod-label:app:model-server
    - k8s:sa:inference-sa
```

The binding status should report `Ready=True` and `Conflict=False`.

## Collision Example

If another `PerObjective` binding in the same namespace resolves to the same pool selector and uses the same container discriminator, both bindings currently enter conflict.

Example conflicting binding:

```yaml
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: objective-b
  namespace: default
spec:
  targetRef:
    name: objective-b
  selectorSource: DerivedFromPool
  workloadSelectorTemplates:
    - k8s:ns:default
    - k8s:sa:inference-sa
  mode: PerObjective
  containerDiscriminator:
    type: ContainerName
    value: main
```

Expected conflict outcome:

- both bindings report `Conflict=True`
- both bindings report `Ready=False`
- managed `ClusterSPIFFEID` resources for the colliding bindings are removed until the collision is fixed
