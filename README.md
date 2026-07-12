<div align="center">
  <img src="docs/assets/images/sondrd-128.png" alt="Sonda Red logo" width="96" height="96">
  <h1>kleym</h1>
  <p><strong>Compile inference identity intent into deterministic SPIFFE identities for Kubernetes.</strong></p>
  <p>
    <a href="https://kleym.sonda.red">Documentation</a>
    ·
    <a href="docs/spec/_index.md">Specs</a>
    ·
    <a href="docs/examples/">Examples</a>
    ·
    <a href="docs/contributing.md">Contributing</a>
  </p>
</div>

<p align="center">
  <a href="https://github.com/sonda-red/kleym/actions/workflows/ci.yml">
    <img src="https://github.com/sonda-red/kleym/actions/workflows/ci.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/sonda-red/kleym/actions/workflows/docs.yml">
    <img src="https://github.com/sonda-red/kleym/actions/workflows/docs.yml/badge.svg" alt="Docs">
  </a>
  <img src="https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+">
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-Apache%202.0-black" alt="License: Apache-2.0">
  </a>
</p>

Kleym connects Gateway API Inference Extension (GAIE) [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) resources to SPIFFE workload identity for Kubernetes.

The in-cluster `kleym-operator` watches `InferencePool` workload intent, then compiles that intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources. The companion `kleym` CLI is a read-only inspection tool for the rendered identity state.

## Where kleym fits

- The [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/) describes inference workloads in Kubernetes.
- `kleym-operator` turns that intent into workload identity registrations constrained by namespace, service account, pool, and a reserved identity-boundary label.
- SPIRE Controller Manager translates managed `ClusterSPIFFEID` resources into SPIRE registration entries; SPIRE Server issues SVIDs, while SPIRE Agent attests workloads and delivers credentials.

## Why kleym

- Derives stable SPIFFE identities from declared pool, service-account, and workload-variant intent.
- Keeps selector rendering workload-constrained with mandatory namespace, service account, pool-derived, and identity-boundary selectors.
- Writes `ClusterSPIFFEID` intent for SPIRE Controller Manager instead of writing SPIRE registration entries or issuing credentials.

## Scope boundary

Kleym stops at identity registration. `kleym-operator` does not deploy inference workloads, route inference traffic, or evaluate request policy.

## How it works

- `InferenceIdentityBinding` declares identity intent and a reserved label boundary for one `InferencePool` workload variant.
- `kleym-operator` resolves the pool to an internal inference target anchored as `pool/<pool-name>`.
- The controller combines that target with the binding namespace, service account, and mandatory identity boundary to render deterministic selectors and variant SPIFFE IDs.
- Peer bindings are evaluated for structural exclusivity before managed output is created or updated.
- Conflicting or duplicate identity claims have their managed `ClusterSPIFFEID` output withdrawn; absence is confirmed before the conflict is reported as settled and before peers may recreate output.

## Quickstart

Prerequisites:

- Go `1.26+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster with the Gateway API Inference Extension [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) CRD
- SPIRE Controller Manager with the `ClusterSPIFFEID` CRD
- Cluster admission policy that controls assignment of the `identity.kleym.sonda.red/variant` Pod label and prevents changing it during a Pod's lifetime; see [`docs/install.md`](docs/install.md#identity-boundary-admission-policy)
- Docker for Kind-backed e2e; the e2e targets bootstrap `kind` and Chainsaw under `bin/`

Run the controller locally:

```sh
make run
```

`make run` defaults to `TRUST_DOMAIN=kleym.sonda.red`. Override it when
running against a SPIRE install with a different trust domain:

```sh
make run TRUST_DOMAIN=example.org CLUSTERSPIFFEID_CLASS_NAME=kleym
```

Install CRDs and deploy the controller:

```sh
make install
make deploy IMG=ghcr.io/sonda-red/kleym-operator:latest
```

Install the latest kleym operator from the root GitOps path:

```sh
kubectl apply -k https://github.com/sonda-red/kleym//deployment?ref=main
```

For release-pinned installs, pin the manifest ref and controller image tag
together. See [`docs/install.md`](docs/install.md) for
Kustomize, Flux, and Argo CD examples.

Run validation:

```sh
make test
make lint
make test-e2e-chainsaw KEEP_KIND=true
```

Build the inspection CLI:

```sh
make build-cli
bin/kleym inspect binding <name> -n <namespace>
```

Use `-o json` for automation. See [`docs/cli/`](docs/cli/) for CLI usage,
results, inspection report fields, findings, and exit codes.

## Reconcile Flow

```mermaid
---
config:
  layout: elk
---
flowchart TD
    B["InferenceIdentityBinding"]

    subgraph GAIE["Gateway API Inference Extension Inputs"]
        P["InferencePool"]
    end

    subgraph Reconcile["kleym-operator Reconcile"]
        D1{"Deleted?"}
        D1Y["Clean up ClusterSPIFFEIDs\nRemove finalizer"]
        F["Ensure finalizer"]
        RESOLVE["Resolve poolRef → Pool"]
        RENDER["Derive selectors from pool\nValidate mandatory boundary + selectors\nRender SPIFFE ID"]
        PEERS{"Exclusive from peers?"}
        WITHDRAW["Withdraw conflict output\nConfirm absence"]
        APPLY["Reconcile ClusterSPIFFEID"]
        STATUS["Patch status + emit events"]
    end

    subgraph SPIRE["SPIRE Stack"]
        CS["ClusterSPIFFEID"]
        SCM["SPIRE Controller Manager"]
        SR["SPIRE registration entries"]
    end

    B --> D1
    D1 -->|yes| D1Y
    D1 -->|no| F --> RESOLVE
    P --> RESOLVE
    RESOLVE --> RENDER --> PEERS
    PEERS -->|yes| APPLY --> STATUS
    PEERS -->|no| WITHDRAW --> STATUS
    APPLY --> CS --> SCM --> SR
    STATUS --> B

    classDef binding fill:#fee2e2,stroke:#b91c1c,color:#7f1d1d,stroke-width:1.4px
    classDef gaie fill:#fff7ed,stroke:#c2410c,color:#7c2d12,stroke-width:1.2px
    classDef controller fill:#f1f5f9,stroke:#475569,color:#0f172a,stroke-width:1.2px
    classDef gate fill:#e2e8f0,stroke:#334155,color:#0f172a,stroke-width:1.2px
    classDef status fill:#dcfce7,stroke:#15803d,color:#14532d,stroke-width:1.2px
    classDef warning fill:#fee2e2,stroke:#dc2626,color:#7f1d1d,stroke-width:1.2px,stroke-dasharray:4 2
    classDef spire fill:#eff6ff,stroke:#1d4ed8,color:#1e3a8a,stroke-width:1.2px

    class B binding
    class P gaie
    class D1 gate
    class D1Y,F,RESOLVE,RENDER,APPLY,WITHDRAW controller
    class PEERS gate
    class STATUS status
    class CS,SCM,SR spire
```

## Documentation

Docs live under [`docs/`](docs/), with the published site at <https://kleym.sonda.red>.

| Topic | What it covers |
| --- | --- |
| [`docs/install.md`](docs/install.md) | Local run, deployment, GitOps install, metrics, and validation commands |
| [`docs/concepts.md`](docs/concepts.md) | GAIE pool input, service-account-scoped inference target identity, and selector safety |
| [`docs/architecture.md`](docs/architecture.md) | End-to-end controller flow |
| [`docs/demo.md`](docs/demo.md) | Reference binding-to-`ClusterSPIFFEID` walkthrough |
| [`docs/examples/`](docs/examples/) | Concrete manifests and expected outcomes |
| [`docs/reference/`](docs/reference/) | API fields, conditions, managed resources, compatibility, dependencies, and GAIE compatibility |
| [`docs/troubleshooting.md`](docs/troubleshooting.md) | Binding conditions, missing CRDs, and selector triage |
| [`docs/design/`](docs/design/) | Controller design notes and downstream handoff patterns |
| [`docs/cli/`](docs/cli/) | CLI usage, results, inspection report, findings, and exit codes |
| [`docs/spec/operator.md`](docs/spec/operator.md) | Authoritative operator product, API, and reconciliation behavior |
| [`docs/spec/cli.md`](docs/spec/cli.md) | Read-only inspection CLI contract |
| [`docs/contributing.md`](docs/contributing.md) | Contributor workflow and validation expectations |

Preview the docs site locally:

```sh
make docs-serve
```

Docs commands require Hugo Extended `0.146+`.

Build the static docs site:

```sh
make docs-build
```

## License

Apache-2.0
