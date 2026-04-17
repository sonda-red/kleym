---
title: Basic Binding
weight: 10
---

This example shows the simplest `PoolOnly` flow.

`kleym` currently consumes only a small slice of the referenced GAIE objects:

- from the objective: `spec.poolRef`
- from the pool: `spec.selector`

Your installed GAIE version may require additional fields on those objects. The snippets below focus on the fields that matter to `kleym`.

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
  name: objective-a-pool
  namespace: default
spec:
  targetRef:
    name: objective-a
  selectorSource: DerivedFromPool
  workloadSelectorTemplates:
    - k8s:ns:default
    - k8s:sa:inference-sa
  mode: PoolOnly
```

## Expected Outcome

The binding should reconcile to a managed `ClusterSPIFFEID` with:

- SPIFFE ID `spiffe://kleym.sonda.red/ns/default/pool/pool-a`
- a pod selector equivalent to `matchLabels.app=model-server`
- workload selectors including:
  - `k8s:ns:default`
  - `k8s:sa:inference-sa`
  - `k8s:pod-label:app:model-server`

The generated `ClusterSPIFFEID` name is deterministic but includes a hash suffix, so the example below focuses on the meaningful fields:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  labels:
    kleym.sonda.red/managed-by: kleym
    kleym.sonda.red/binding-name: objective-a-pool
    kleym.sonda.red/binding-namespace: default
spec:
  spiffeIDTemplate: spiffe://kleym.sonda.red/ns/default/pool/pool-a
  podSelector:
    matchLabels:
      app: model-server
  workloadSelectorTemplates:
    - k8s:ns:default
    - k8s:pod-label:app:model-server
    - k8s:sa:inference-sa
```

The binding status should report:

- `Ready=True`
- `Conflict=False`
- `InvalidRef=False`
- `UnsafeSelector=False`
- `RenderFailure=False`
