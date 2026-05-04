# Reference Inference Environment

This directory contains a small reference inference environment that users,
tests, and demo docs can reuse before applying an `InferenceIdentityBinding`.
It gives `kleym` something realistic and deterministic to bind to: a namespace,
service account, serving workload, `InferencePool`, and `InferenceObjective`
with stable names and selectors.

These manifests are helpful when you want to verify the base `kleym` flow
without bringing a full inference stack, gateway, route, or policy layer:

1. Apply this reference environment.
2. Apply an `InferenceIdentityBinding` that targets `reference-objective`.
3. Check that `kleym` renders the expected managed `ClusterSPIFFEID`.

The resources in this directory model inputs that already exist in the cluster.
They are externally owned, not part of the default `kleym` install path, not
Helm-style product templates, and not resources that `kleym` creates, modifies,
or reconciles.

## Included Resources

| Resource | Name | Purpose |
| --- | --- | --- |
| `Namespace` | `kleym-reference-inference` | Stable namespace for `k8s:ns` selector tests. |
| `ServiceAccount` | `reference-inference` | Stable service account for `k8s:sa` selector tests. |
| `Deployment` | `reference-model-server` | Minimal externally managed workload input. |
| `InferencePool` | `reference-pool` | GAIE pool that selects the workload pods. |
| `InferenceObjective` | `reference-objective` | GAIE objective that references the pool. |

## Stable Selector Values

Tests and demos can rely on these values:

| Field | Value |
| --- | --- |
| Namespace | `kleym-reference-inference` |
| Service account | `reference-inference` |
| Pod label | `app.kubernetes.io/name=reference-model-server` |
| Pod label | `app.kubernetes.io/part-of=kleym-reference-inference` |
| Container name | `model-server` |
| Container port | `8000` |

An `InferenceIdentityBinding` that targets this reference environment should be
applied by the test or demo layer, not by this directory. For `PerObjective`
mode, use `containerDiscriminator.type: ContainerName` and
`containerDiscriminator.value: model-server`.

## Not Included

This reference environment intentionally does not include:

- `InferenceIdentityBinding` resources
- managed `ClusterSPIFFEID` resources
- Envoy, OPA, Gateway API, Envoy Gateway, route, policy, SDS, or downstream
  enforcement resources

Those layers are separate from the reference inference environment and should be
added only by tests or demos that explicitly need them.

Apply the reference manifests with:

```sh
kubectl apply -k test/reference/inference-environment
```
