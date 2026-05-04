<div align="center">
  <img src="docs/assets/images/sondrd-128.png" alt="Sonda Red logo" width="96" height="96">
  <h1>kleym</h1>
  <p><strong>Compile inference identity intent into deterministic SPIFFE identities for Kubernetes.</strong></p>
  <p>
    <a href="https://kleym.sonda.red">Documentation</a>
    ·
    <a href="docs/spec.md">Spec</a>
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

`kleym` is a Kubernetes operator for clusters that use the [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/). It reads inference intent from resources such as [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/), then compiles that intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources.

## Where kleym fits

- The [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/) describes inference workloads and request objectives in Kubernetes.
- `kleym` turns that intent into workload identity registrations with tenant-safe selectors.
- SPIRE Controller Manager applies those registrations so SPIRE can issue identities to the matching workloads.

## Why kleym

- Derives stable SPIFFE identities from Gateway API Inference Extension resources instead of ad hoc labels.
- Keeps selector rendering tenant-safe by intersecting namespace, service account, pool-derived selectors, and optional container discrimination.
- Delegates identity issuance and rotation to SPIRE Controller Manager instead of writing SPIRE entries directly.

## Scope boundary

`kleym` is an identity registration compiler. It does not deploy inference workloads, route inference traffic, or evaluate request policy.

## How it works

- `InferenceIdentityBinding` declares identity intent for one `InferenceObjective`.
- `kleym` resolves that objective and its referenced `InferencePool`.
- The controller renders deterministic selectors and SPIFFE IDs from those inputs.
- Managed `ClusterSPIFFEID` resources are reconciled for SPIRE Controller Manager.

## Quickstart

Prerequisites:

- Go `1.26+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster with the Gateway API Inference Extension [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) CRDs
- SPIRE Controller Manager with the `ClusterSPIFFEID` CRD
- `kind` for `make test-e2e`

Run the controller locally:

```sh
make run
```

Install CRDs and deploy the controller:

```sh
make install
make deploy IMG=ghcr.io/sonda-red/kleym:latest
```

Run validation:

```sh
make test
make lint
```

## Reconcile Flow

```mermaid
---
config:
  layout: elk
---
flowchart TD
    B["InferenceIdentityBinding"]

    subgraph GAIE["Gateway API Inference Extension Inputs"]
        O["InferenceObjective"]
        P["InferencePool"]
    end

    subgraph Reconcile["kleym Reconcile"]
        D1{"Deleted?"}
        D1Y["Clean up ClusterSPIFFEIDs\nRemove finalizer"]
        F["Ensure finalizer"]
        RESOLVE["Resolve targetRef → Objective\nExtract and resolve poolRef → Pool"]
        RENDER["Derive selectors from pool\nAdd container discriminator (PerObjective)\nValidate safety selectors\nRender SPIFFE ID"]
        COL{"Collision?"}
        COLY["Set Conflict status\nClean up ClusterSPIFFEIDs"]
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
    O & P --> RESOLVE
    RESOLVE --> RENDER --> COL
    COL -->|yes| COLY --> STATUS
    COL -->|no| APPLY --> STATUS
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
    class O,P gaie
    class D1,COL gate
    class D1Y,F,RESOLVE,RENDER,APPLY controller
    class STATUS status
    class COLY warning
    class CS,SCM,SR spire
```

## Documentation

Docs live under [`docs/`](docs/), with the published site at <https://kleym.sonda.red>.

| Topic | What it covers |
| --- | --- |
| [`docs/install.md`](docs/install.md) | Local run, deployment, and test commands |
| [`docs/concepts.md`](docs/concepts.md) | Identity boundaries, selector safety, and scope |
| [`docs/architecture.md`](docs/architecture.md) | End-to-end controller flow |
| [`docs/demo.md`](docs/demo.md) | Reproducible binding-to-`ClusterSPIFFEID` walkthrough |
| [`docs/examples/`](docs/examples/) | Concrete manifests and expected outcomes |
| [`docs/reference/`](docs/reference/) | API surface, conditions, and managed resources |
| [`docs/troubleshooting.md`](docs/troubleshooting.md) | Condition-driven debugging and dependency checks |
| [`docs/versioning.md`](docs/versioning.md) | Docs version snapshot workflow |
| [`docs/spec.md`](docs/spec.md) | Authoritative product and API behavior |
| [`docs/design/`](docs/design/) | Internal design notes |
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

Build root and configured version snapshots:

```sh
make docs-build-versioned
```

## License

Apache-2.0
