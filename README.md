# kleym

`kleym` is a Kubernetes operator that turns inference intent into deterministic [SPIFFE](https://spiffe.io/) identities.

It is designed to read [GAIE](https://gateway-api-inference-extension.sigs.k8s.io/) resources and reconcile identity resources via [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager).

`kleym` does not deploy inference workloads, route inference traffic, or enforce request policy.

## Why

Inference stacks are increasingly standardized, but identity registration is still often manual and inconsistent.

`kleym` aims to bridge that gap by converting inference metadata into stable workload/model identities with tenant-safe selector derivation.

## Status

This repository is in early scaffold stage.

- Product direction and MVP behavior are documented in `docs/spec.md`.
- API group and CRD scaffold exist for `InferenceTrustBinding` (`kleym.sonda.red/v1alpha1`).

## Planned MVP (Design Target)

- Consume GAIE `InferenceObjective` and `InferencePool`.
- Reconcile one or more `ClusterSPIFFEID` resources per binding.
- Support pool-level and per-objective identities.
- Enforce namespace and selector safety constraints.

See `docs/spec.md` for the full specification.

## Quickstart (Current Scaffold)

Prerequisites:

- Go `1.24+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster

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
