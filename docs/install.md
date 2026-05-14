---
title: Install
weight: 30
---

This page covers the practical commands for running `kleym-operator`, deploying it, testing it, and previewing the documentation site locally.

## Prerequisites

- Go `1.26+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster
- Gateway API Inference Extension (GAIE) CRD for [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/), plus [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) when using `PerObjective`
- SPIFFE Runtime Environment (SPIRE) Controller Manager with the [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
- Docker for Kind-backed e2e; the e2e targets bootstrap `kind` and Chainsaw under `bin/`
- Hugo Extended `0.146+` for docs preview/build

The repository bootstraps local tool binaries under `bin/` through `make` targets, so you do not need to install `controller-gen`, `kustomize`, `setup-envtest`, `golangci-lint`, `kind`, or Chainsaw globally.

For identity-system background, see [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/) and [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/).

## Run Locally

Run the controller against your current kubeconfig:

```sh
make run
```

Build the operator binary:

```sh
make build-operator
```

## Deploy

Install the CRD into the current cluster:

```sh
make install
```

Deploy the controller image:

```sh
make deploy IMG=<registry>/kleym-operator:<tag>
```

Render the local consolidated installer manifest into `dist/install.yaml`:

```sh
make build-installer
```

`dist/install.yaml` is generated output and is not committed to the repository.

For Kustomize, Flux, or Argo CD installs, use the root `deployment/`
kustomization described below.

## GitOps Install

Use the root `deployment/` kustomization when Flux or Argo CD should manage the
kleym operator.

The path installs the kleym CRD, namespace, RBAC, controller Deployment, and
metrics Service. It does not install external dependency CRDs: Gateway API
Inference Extension CRDs and SPIRE Controller Manager, including the
`ClusterSPIFFEID` CRD, must already be installed.

Install the latest operator manifests directly from `main`:

```sh
kubectl apply -k https://github.com/sonda-red/kleym//deployment?ref=main
```

For release-pinned installs, pin both the manifest ref and controller image tag
with a Kustomize overlay:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- https://github.com/sonda-red/kleym//deployment?ref=vX.Y.Z
images:
- name: ghcr.io/sonda-red/kleym-operator
  newTag: vX.Y.Z
```

Apply that overlay:

```sh
kubectl apply -k .
```

Flux example using latest manifests from `main`:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: kleym
  namespace: flux-system
spec:
  interval: 1h
  url: https://github.com/sonda-red/kleym
  ref:
    branch: main
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kleym
  namespace: flux-system
spec:
  interval: 1h
  path: ./deployment
  prune: false
  sourceRef:
    kind: GitRepository
    name: kleym
  wait: true
```

Argo CD example using latest manifests from `main`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kleym
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/sonda-red/kleym.git
    targetRevision: main
    path: deployment
  destination:
    server: https://kubernetes.default.svc
    namespace: kleym-system
  syncPolicy:
    automated:
      prune: false
      selfHeal: true
```

The examples leave pruning disabled because deleting a CRD also deletes its
custom resources.

Pinned Flux example using release manifests and a matching controller image tag:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: kleym
  namespace: flux-system
spec:
  interval: 1h
  url: https://github.com/sonda-red/kleym
  ref:
    tag: vX.Y.Z
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kleym
  namespace: flux-system
spec:
  interval: 1h
  path: ./deployment
  prune: false
  sourceRef:
    kind: GitRepository
    name: kleym
  images:
  - name: ghcr.io/sonda-red/kleym-operator
    newTag: vX.Y.Z
  wait: true
```

Pinned Argo CD example using release manifests and a matching controller image
tag:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kleym
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/sonda-red/kleym.git
    targetRevision: vX.Y.Z
    path: deployment
    kustomize:
      images:
      - ghcr.io/sonda-red/kleym-operator:vX.Y.Z
  destination:
    server: https://kubernetes.default.svc
    namespace: kleym-system
  syncPolicy:
    automated:
      prune: false
      selfHeal: true
```

A raw commit SHA pins manifest content at that commit. Use an image override
when the controller image must also be pinned.

Helm is not needed for this installation path. The manifests are static, GitOps
tools can pin the manifest ref and image tag directly, and Flux or Argo CD can
consume the kustomization without a chart. A chart should be revisited when
kleym needs a larger templated install surface for operator-specific options.

## Test

Run controller and API tests:

```sh
make test
```

Run lint:

```sh
make lint
```

Run the Kind-backed Chainsaw reconciliation check (primary e2e path):

```sh
make test-e2e-chainsaw
```

Keep the Kind cluster for faster local iteration:

```sh
make test-e2e-chainsaw KEEP_KIND=true
```

Use the smallest command set that proves the change. See [contributing](contributing) for the repository validation expectations.

## Preview Docs

Serve the Hextra docs site locally:

```sh
make docs-serve
```

Build the static site locally:

```sh
make docs-build
```

Override the port if you need something other than `1313`:

```sh
make docs-serve DOCS_PORT=8080
```
