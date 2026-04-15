# kleym

`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources.

`kleym` is an identity registration compiler. It does not deploy inference workloads, route inference traffic, or evaluate request policy.

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

Project docs are organized by reader intent and live under [`docs/`](docs/).

- [`docs/index.md`](docs/index.md): landing page
- [`docs/spec.md`](docs/spec.md): the authoritative behavioral contract
- [`docs/roadmap.md`](docs/roadmap.md): planned milestones and sequencing
- [`docs/install.md`](docs/install.md): local run, deploy, and test commands
- [`docs/reference/`](docs/reference): stable facts about API surface, conditions, and managed resources
- [`docs/design/`](docs/design): internal design notes
- [`docs/examples/`](docs/examples): concrete manifests and expected outcomes
- [`docs/contributing.md`](docs/contributing.md): contributor workflow and validation expectations

Preview the docs site locally:

```sh
make docs-serve
```

Build the static docs site:

```sh
make docs-build
```

## License

Apache-2.0
