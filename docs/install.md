# Install

This page covers the practical commands for running `kleym`, deploying it, testing it, and previewing the documentation site locally.

## Prerequisites

- Go `1.25+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster
- `kind` for `make test-e2e`

The repository bootstraps local tool binaries under `bin/` through `make` targets, so you do not need to install `controller-gen`, `kustomize`, `setup-envtest`, or `golangci-lint` globally.

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

Use the smallest command set that proves the change. See [contributing](contributing.md) for the repository validation expectations.

## Preview Docs

Serve the MkDocs site with the pinned container image:

```sh
make docs-serve
```

Build the static site locally:

```sh
make docs-build
```

Override the port if you need something other than `8000`:

```sh
make docs-serve DOCS_PORT=8080
```

If you prefer a local Python workflow instead of the containerized one, `requirements-docs.txt` still pins the MkDocs dependencies for that path.
