# kleym Documentation

`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic SPIFFE identities.

## Overview

- Read [concepts](concepts.md) for the identity model, mode choices, and safety boundaries.
- Read [architecture](architecture.md) for the end-to-end control flow from `InferenceIdentityBinding` to `ClusterSPIFFEID`.

## Use

- Read [install](install.md) to run the controller locally, deploy it, and execute validation commands.
- Read [examples](examples/basic-binding.md) for concrete manifests and expected outcomes.
- Read [reference](reference/api.md) for stable facts about the API, conditions, and managed resources.
- Read [troubleshooting](troubleshooting.md) when reconciliation fails or cluster dependencies are missing.

## Design

- Read [spec](spec.md) for the authoritative behavior contract.
- Read [reconciliation](design/reconciliation.md), [selector safety](design/selector-safety.md), and [collision detection](design/collision-detection.md) for controller internals.
- Read [roadmap](roadmap.md) for planned milestones and sequencing.

## Development

- Read [contributing](contributing.md) if you are changing the repository.

Documentation rules for this repo:

- `spec.md` stays singular and authoritative.
- overview pages summarize the model without duplicating the full contract.
- `reference/` states what is true today.
- `design/` explains why the controller works the way it does.
- issues and pull request text are design history, not documentation.
