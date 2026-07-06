---
title: Demo
weight: 35
description: "A reference walkthrough that shows how an InferenceIdentityBinding reconciles into deterministic ClusterSPIFFEID output."
aliases:
  - /operator/demo/
---

This demo verifies the base identity attachment flow:

`InferencePool` -> `InferenceIdentityBinding` -> managed `ClusterSPIFFEID`

It reuses the reference inference environment from
`test/reference/inference-environment/`. Those manifests are externally owned
inputs: `kleym-operator` does not create, modify, or manage the workload, pool,
gateway, route, or policy layer.

## Scope

This demo proves that `kleym-operator` reconciles deterministic registration output. It
does not prove request-time authorization, mTLS enforcement, or SVID
consumption by a gateway, mesh, proxy, or application.

mTLS enforcement is external to `kleym-operator`. For downstream consumption
patterns, read [Downstream Patterns](/design/downstream-patterns/).

## Prerequisites

Use a Kubernetes cluster with these dependencies already installed:

- Gateway API Inference Extension CRD for the reference `InferencePool`
- SPIRE Controller Manager with the `ClusterSPIFFEID` CRD
- `kleym-operator` installed and running

Confirm the external CRDs and controller are present:

```sh
kubectl get crd inferencepools.inference.networking.k8s.io
kubectl get crd clusterspiffeids.spire.spiffe.io
kubectl -n kleym-system rollout status deployment/kleym-operator --timeout=120s
```

Expected observation: the CRDs exist and the controller deployment is available.

If `kleym-operator` is not installed yet, use the install commands in
[Install](/install/).

## Apply The Reference Environment

Apply the externally owned workload and GAIE inputs from the reference fixture:

```sh
kubectl apply -k test/reference/inference-environment
kubectl -n kleym-reference-inference rollout status deployment/reference-model-server --timeout=120s
```

Expected observation: the reference namespace, service account, workload, and
`InferencePool` exist before any binding is applied.

## Apply The Binding

Apply an `InferenceIdentityBinding` that anchors to the reference pool:

```sh
kubectl apply -f - <<'EOF'
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: reference-pool-binding
  namespace: kleym-reference-inference
spec:
  poolRef:
    name: reference-pool
  serviceAccountName: reference-inference
EOF
```

Wait for reconciliation:

```sh
kubectl -n kleym-reference-inference wait \
  --for=condition=Ready \
  inferenceidentitybinding/reference-pool-binding \
  --timeout=120s
```

Expected observation: the binding reaches `Ready=True`.

Confirm the success conditions:

```sh
kubectl -n kleym-reference-inference get inferenceidentitybinding reference-pool-binding \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
```

Expected observation:

```text
Ready=True Reconciled
InvalidRef=False Resolved
UnsafeSelector=False Resolved
RenderFailure=False Resolved
```

## Inspect Managed Output

Inspect the managed `ClusterSPIFFEID`:

```sh
kubectl get clusterspiffeids.spire.spiffe.io \
  -l kleym.sonda.red/binding-name=reference-pool-binding,kleym.sonda.red/binding-namespace=kleym-reference-inference \
  -o yaml
```

Expected observation: exactly one managed `ClusterSPIFFEID` exists. Its
`spec.spiffeIDTemplate` is
`spiffe://kleym.sonda.red/ns/kleym-reference-inference/pool/reference-pool`,
its pod selector matches the reference pool selector, and its workload selectors
include the reference namespace, service account, and pool labels.

## Check Stable Reconcile

Capture the managed object name, then reapply the same inputs:

```sh
CLUSTERSPIFFEID_NAME="$(kubectl get clusterspiffeids.spire.spiffe.io \
  -l kleym.sonda.red/binding-name=reference-pool-binding,kleym.sonda.red/binding-namespace=kleym-reference-inference \
  -o jsonpath='{.items[0].metadata.name}')"

kubectl apply -k test/reference/inference-environment
kubectl get clusterspiffeids.spire.spiffe.io "$CLUSTERSPIFFEID_NAME"
kubectl -n kleym-reference-inference wait \
  --for=condition=Ready \
  inferenceidentitybinding/reference-pool-binding \
  --timeout=120s
```

Expected observation: the same `ClusterSPIFFEID` remains present and the binding
stays `Ready=True`.

For detailed field-level examples, read [Examples](/examples/).

## Clean Up

Delete the binding first so `kleym-operator` can remove its managed output:

```sh
kubectl -n kleym-reference-inference delete inferenceidentitybinding reference-pool-binding
kubectl wait --for=delete clusterspiffeids.spire.spiffe.io "$CLUSTERSPIFFEID_NAME" --timeout=120s
kubectl delete -k test/reference/inference-environment
```

Expected observation: the managed `ClusterSPIFFEID` is removed before the
reference environment is deleted.
