# kleym

`kleym` is a Kubernetes operator that makes inference workloads legible to workload identity by translating inference intent into deterministic [SPIFFE](https://spiffe.io/) identities.

It compiles identity intent into [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager) resources, primarily [`ClusterSPIFFEID`][clusterspiffeid].

## Scope Boundary

`kleym` is an identity registration compiler.

It does not:

- Deploy inference workloads
- Route inference traffic
- Evaluate request policy

Those concerns remain in the inference stack, gateway, mesh, or external policy engines.

## Why

Inference stacks can be deployed reliably, but identity registration is still often manual and inconsistent across teams.

[Gateway API Inference Extension (GAIE)](https://gateway-api-inference-extension.sigs.k8s.io/) introduces inference-specific API objects, but not identity semantics. `kleym` bridges that gap by deriving stable SPIFFE ID templates and tenant-safe selectors from GAIE resources.

## Core Value

- Deterministic identities derived from GAIE metadata rather than ad hoc labels.
- A single namespaced control surface for identity intent across heterogeneous inference stacks that share GAIE semantics.
- Low operational risk by delegating issuance and rotation to SPIRE Controller Manager.

## Preferred Inference Signal

GAIE v1 objects are the primary signal:

- [`InferenceObjective`][gaie-inferenceobjective] is the primary model-level object and references an [`InferencePool`][gaie-inferencepool] via `poolRef`.
- [`InferencePool`][gaie-inferencepool] defines the serving pod pool for inference traffic.
- [`InferenceModel`][gaie-inferencemodel-legacy] is treated as legacy.

## Identity Model

- Pool identity: one SPIFFE identity representing serving pool pods.
- Objective identity: one SPIFFE identity per [`InferenceObjective`][gaie-inferenceobjective], including when multiple objectives share the same pool.

## MVP Design Target

- External CRDs consumed:
  - GAIE [`InferencePool`][gaie-inferencepool]
  - GAIE [`InferenceObjective`][gaie-inferenceobjective]
- `kleym` CRD:
  - `InferenceIdentityBinding` (`kleym.sonda.red/v1alpha1`) expresses identity intent for one `InferenceObjective`.
- Controller behavior:
  - Resolve `targetRef` (`InferenceObjective`) and `poolRef` (`InferencePool`).
  - Derive pod selectors from pool intent and intersect with required safety templates.
  - Reconcile one or more [`ClusterSPIFFEID`][clusterspiffeid] resources with computed SPIFFE IDs.
  - Set status conditions such as `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, `RenderFailure`.

See `docs/spec.md` for the complete and authoritative specification.

## Current Status

This repository is in early scaffold stage.

- Product direction and MVP behavior are documented in `docs/spec.md`.
- API group and CRD scaffold exist for `InferenceIdentityBinding` (`kleym.sonda.red/v1alpha1`).

## Documentation Map

- [`docs/spec.md`](docs/spec.md): authoritative product and API behavior.
- [`CONTRIBUTING.md`](CONTRIBUTING.md): local development workflow, repository layout, and testing expectations.
- [`AGENTS.md`](AGENTS.md): minimal repository instructions for coding agents.
- [`SEMANTIC_VERSIONING.md`](SEMANTIC_VERSIONING.md): release automation and commit conventions.
- [`CHANGELOG.md`](CHANGELOG.md): released changes.

## Dependencies

- [SPIRE Server](https://spiffe.io/spire/) and SPIRE Agent
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager) with [`ClusterSPIFFEID`][clusterspiffeid]

## Quickstart (Current Scaffold)

Prerequisites:

- Go `1.25+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster
- `kind` for `make test-e2e`

Run locally:

```sh
make run
```

Install CRD and deploy controller:

```sh
make install
make deploy IMG=<registry>/kleym:<tag>
```

Run tests:

```sh
make test
```

## License

Apache-2.0

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[gaie-inferenceobjective]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/
[gaie-inferencemodel-legacy]: https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/
