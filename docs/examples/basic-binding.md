---
title: Basic Binding
weight: 10
description: "Basic InferenceIdentityBinding example that renders one label-bound workload variant identity and managed ClusterSPIFFEID resource."
aliases:
  - /operator/examples/basic-binding/
---

This example shows a service-account-scoped identity anchored to a GAIE pool.

`kleym-operator` currently consumes only a small slice of the referenced Gateway API Inference Extension (GAIE) objects:

- from the pool: `spec.selector`

Your installed GAIE version may require additional fields on that object. The snippets below focus on the fields that matter to `kleym-operator`.
For full GAIE schema details, see [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/).
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
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: pool-a
  namespace: default
spec:
  poolRef:
    name: pool-a
  serviceAccountName: inference-sa
  identityBoundary:
    labelKey: identity.kleym.sonda.red/variant
    labelValue: prefill
```

The selected Pods must carry
`identity.kleym.sonda.red/variant=prefill`. Apply this example only after the
cluster has the required
[identity-boundary admission policy](/install/#identity-boundary-admission-policy);
Kleym neither adds the label nor enforces its Pod-lifetime immutability.

## Expected Outcome

The binding should reconcile to a managed `ClusterSPIFFEID` with:

- SPIFFE ID `spiffe://kleym.sonda.red/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill`
- a pod selector equivalent to `matchLabels.app=model-server`
- workload selectors including:
  - `k8s:ns:default`
  - `k8s:sa:inference-sa`
  - `k8s:pod-label:app:model-server`
  - `k8s:pod-label:identity.kleym.sonda.red/variant:prefill`

The generated `ClusterSPIFFEID` name is deterministic but includes a hash suffix, so the example below focuses on the meaningful fields:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  labels:
    kleym.sonda.red/managed-by: kleym
    kleym.sonda.red/binding-name: pool-a
    kleym.sonda.red/binding-namespace: default
spec:
  spiffeIDTemplate: spiffe://kleym.sonda.red/ns/default/sa/inference-sa/inference/pool/pool-a/variant/prefill
  podSelector:
    matchLabels:
      app: model-server
  workloadSelectorTemplates:
    - k8s:ns:default
    - k8s:pod-label:app:model-server
    - k8s:pod-label:identity.kleym.sonda.red/variant:prefill
    - k8s:sa:inference-sa
```

The binding status should report:

- `Ready=True`
- `InvalidRef=False`
- `UnsafeSelector=False`
- `Conflict=False`
- `RenderFailure=False`
