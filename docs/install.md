---
title: Install
weight: 30
---

This page covers the practical commands for running `kleym`, deploying it, testing it, and previewing the documentation site locally.

## Prerequisites

- Go `1.25+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster
- Gateway API Inference Extension (GAIE) CRDs for [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- SPIFFE Runtime Environment (SPIRE) Controller Manager with the [`ClusterSPIFFEID` CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
- `kind` for `make test-e2e`
- Hugo Extended `0.146+` for docs preview/build

The repository bootstraps local tool binaries under `bin/` through `make` targets, so you do not need to install `controller-gen`, `kustomize`, `setup-envtest`, or `golangci-lint` globally.

For identity-system background, see [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/) and [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/).

## Run Locally

Run the controller against your current kubeconfig:

```sh
make run
```

Build the manager binary:

```sh
make build
```

## Deploy

Install the CRD into the current cluster:

```sh
make install
```

Deploy the controller image:

```sh
make deploy IMG=<registry>/kleym:<tag>
```

Render the consolidated installer manifest:

```sh
make build-installer
```

## Test

Run controller and API tests:

```sh
make test
```

Run lint:

```sh
make lint
```

Run Kind-backed end-to-end coverage:

```sh
make test-e2e
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

Build the default site plus configured version snapshots:

```sh
make docs-build-versioned
```

Override the port if you need something other than `1313`:

```sh
make docs-serve DOCS_PORT=8080
```

Version snapshots are defined in `.docs-versions`.
