# kleym

`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources.

`kleym` is an identity registration compiler. It does not deploy inference workloads, route inference traffic, or evaluate request policy.

## Reconcile Flow

```mermaid
---
config:
  layout: elk
---
flowchart TD
    B["InferenceIdentityBinding"]

    subgraph GAIE["GAIE Inputs"]
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

## Quickstart

Prerequisites:

- Go `1.25+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster
- `kind` for `make test-e2e`

Run the controller locally:

```sh
make run
```

Install CRDs and deploy the controller:

```sh
make install
make deploy IMG=<registry>/kleym:<tag>
```

Run tests:

```sh
make test
make lint
```

## Documentation

Docs live under [`docs/`](docs/).
Published docs: <https://kleym.sonda.red>.

- Overview:
  - [`docs/_index.md`](docs/_index.md): landing page
  - [`docs/concepts.md`](docs/concepts.md): identity model
  - [`docs/architecture.md`](docs/architecture.md): end-to-end controller flow
- Use:
  - [`docs/install.md`](docs/install.md): local run, deploy, and test commands
  - [`docs/examples/`](docs/examples): concrete manifests and expected outcomes
  - [`docs/reference/`](docs/reference): stable facts about API surface, conditions, and managed resources
  - [`docs/troubleshooting.md`](docs/troubleshooting.md): condition-driven debugging and dependency checks
  - [`docs/versioning.md`](docs/versioning.md): docs version snapshot workflow
- Design:
  - [`docs/spec.md`](docs/spec.md): the authoritative behavioral contract
  - [`docs/design/`](docs/design): internal design notes
- Development:
  - [`docs/contributing.md`](docs/contributing.md): contributor workflow and validation expectations

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
