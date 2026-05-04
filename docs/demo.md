---
title: Demo
weight: 35
---

This demo verifies the base identity attachment flow:

`InferencePool` + `InferenceObjective` -> `InferenceIdentityBinding` -> managed `ClusterSPIFFEID`

It reuses the reference inference environment from
`test/reference/inference-environment/`. Those manifests are externally owned
inputs: `kleym` does not create, modify, or manage the workload, pool,
objective, gateway, route, or policy layer.

## Scope

This demo proves that `kleym` reconciles deterministic registration output. It
does not prove request-time authorization, mTLS enforcement, or SVID
consumption by a gateway, mesh, proxy, or application.

mTLS enforcement is external to `kleym`. For downstream consumption patterns,
read [Downstream Enforcement](/reference/downstream-enforcement/).

## Prerequisites

Use a Kubernetes cluster with these dependencies already installed:

- Gateway API Inference Extension CRDs for the reference `InferencePool` and
  `InferenceObjective`
- SPIRE Controller Manager with the `ClusterSPIFFEID` CRD
- `kleym` installed and running

Confirm the external CRDs and controller are present:

```sh
kubectl get crd inferencepools.inference.networking.k8s.io
kubectl get crd inferenceobjectives.inference.networking.x-k8s.io
kubectl get crd clusterspiffeids.spire.spiffe.io
kubectl -n kleym-system rollout status deployment/kleym-controller-manager --timeout=120s
```

Expected observation: the CRDs exist and the controller deployment is available.

If `kleym` is not installed yet, use the install commands in
[Install](/install/).

## Apply The Reference Environment

Apply the externally owned workload and GAIE inputs from the reference fixture:

```sh
kubectl apply -k test/reference/inference-environment
kubectl -n kleym-reference-inference rollout status deployment/reference-model-server --timeout=120s
```

Expected observation: the reference namespace, service account, workload,
`InferencePool`, and `InferenceObjective` exist before any binding is applied.

## Apply The Binding

Apply an `InferenceIdentityBinding` that targets the reference objective:

```sh
kubectl apply -f - <<'EOF'
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: reference-objective-binding
  namespace: kleym-reference-inference
spec:
  targetRef:
    name: reference-objective
  selectorSource: DerivedFromPool
  workloadSelectorTemplates:
    - k8s:ns:kleym-reference-inference
    - k8s:sa:reference-inference
  mode: PerObjective
  containerDiscriminator:
    type: ContainerName
    value: model-server
EOF
```

Wait for reconciliation:

```sh
kubectl -n kleym-reference-inference wait \
  --for=condition=Ready \
  inferenceidentitybinding/reference-objective-binding \
  --timeout=120s
```

Expected observation: the binding reaches `Ready=True`.

Confirm the success conditions:

```sh
kubectl -n kleym-reference-inference get inferenceidentitybinding reference-objective-binding \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
```

Expected observation:

```text
Ready=True Reconciled
Conflict=False Resolved
InvalidRef=False Resolved
UnsafeSelector=False Resolved
RenderFailure=False Resolved
```

## Inspect Managed Output

Inspect the managed `ClusterSPIFFEID`:

```sh
kubectl get clusterspiffeids.spire.spiffe.io \
  -l kleym.sonda.red/binding-name=reference-objective-binding,kleym.sonda.red/binding-namespace=kleym-reference-inference \
  -o yaml
```

Expected observation: exactly one managed `ClusterSPIFFEID` exists. Its
`spec.spiffeIDTemplate` is
`spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective`,
its pod selector matches the reference pool selector, and its workload selectors
include the reference namespace, service account, pool labels, and
`k8s:container-name:model-server`.

## Check Stable Reconcile

Capture the managed object name, then reapply the same inputs:

```sh
CLUSTERSPIFFEID_NAME="$(kubectl get clusterspiffeids.spire.spiffe.io \
  -l kleym.sonda.red/binding-name=reference-objective-binding,kleym.sonda.red/binding-namespace=kleym-reference-inference \
  -o jsonpath='{.items[0].metadata.name}')"

kubectl apply -k test/reference/inference-environment
kubectl get clusterspiffeids.spire.spiffe.io "$CLUSTERSPIFFEID_NAME"
kubectl -n kleym-reference-inference wait \
  --for=condition=Ready \
  inferenceidentitybinding/reference-objective-binding \
  --timeout=120s
```

Expected observation: the same `ClusterSPIFFEID` remains present and the binding
stays `Ready=True`.

For detailed field-level examples, read [Examples](/examples/).

## Clean Up

Delete the binding first so `kleym` can remove its managed output:

```sh
kubectl -n kleym-reference-inference delete inferenceidentitybinding reference-objective-binding
kubectl wait --for=delete clusterspiffeids.spire.spiffe.io "$CLUSTERSPIFFEID_NAME" --timeout=120s
kubectl delete -k test/reference/inference-environment
```

Expected observation: the managed `ClusterSPIFFEID` is removed before the
reference environment is deleted.
