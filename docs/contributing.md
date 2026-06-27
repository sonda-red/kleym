---
title: Contributing
weight: 110
description: "Contributor workflow, repository layout, validation commands, documentation rules, and security expectations for Kleym changes."
---

## Sources Of Truth

- [Operator Spec](/spec/operator/) is the authoritative operator product and API behavior document.
- [CLI Spec](/spec/cli/) is the read-only inspection CLI contract.
- Operator docs live in the default docs section: [Install](/install/), [Concepts](/concepts/), [Architecture](/architecture/), [Reference](/reference/), [Troubleshooting](/troubleshooting/), and [Design](/design/).
- [CLI docs](/cli/) cover user-facing inspection usage, results, report shape, findings, and exit codes.
- `README.md` is the project entry point and quickstart.
- `AGENTS.md` is the minimal repository contract for coding agents.
- `RELEASING.md` describes the manual release procedure.

If code and docs disagree, fix the disagreement instead of silently choosing one.

## Core Stabilization Focus

Prioritize API stability, selector safety, collision behavior, managed output,
finalizer cleanup, and read-only inspection.

## Check GitHub Context First

Issues and pull requests are part of the design history for this repository. Check the directly relevant GitHub context when:

- a task references an issue, PR, review thread, release note, or regression
- intended behavior is unclear from the current code and either the [Operator Spec](/spec/operator/) or [CLI Spec](/spec/cli/)
- you are changing API contracts, reconciliation semantics, CI, release flow, or project policy

Keep that review focused. Read the issue or PR that motivated the change and any directly related follow-ups. Avoid broad history searches unless the task genuinely requires it.

## Ticket Workflow

- If work is tied to an issue, follow the issue instructions explicitly.
- Keep scope tight. If adjacent cleanup is not required, propose a follow-up ticket instead of bundling it in.
- Start the work on a dedicated branch, not on `main`.
- Open a PR for the branch and include an automatic issue-closing reference when an issue number exists, for example `Fixes #123` or `Closes #123`.
- Use the PR description to separate delivered scope from follow-up ideas.

## Development Prerequisites

- Go `1.26+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster for deployment testing
- Docker for local Kind-backed e2e testing; the e2e targets bootstrap `kind` and Chainsaw under `bin/`
- Hugo Extended `0.146+` for docs preview/build

The repository bootstraps local tool binaries under `bin/` through `make` targets, so global installs of `controller-gen`, `kustomize`, `setup-envtest`, `golangci-lint`, `kind`, and Chainsaw are not required.

## Repository Map

- `cmd/kleym-operator/main.go`: operator entry point
- `cmd/kleym/main.go`: CLI entry point
- `api/v1alpha1`: API types and generated deepcopy code for `InferenceIdentityBinding`
- `internal/controller`: reconciliation logic and controller-focused tests
- `internal/cli` and `internal/inspection`: read-only inspection CLI and report generation
- `config/`: CRD, RBAC, operator deployment, samples, and kustomize overlays
- `test/chainsaw`: Chainsaw scenarios for declarative cluster reconciliation checks
- `.github/workflows`: CI, release, and maintenance automation

## Common Commands

- `make help`: list supported targets
- `make docs-build`: build the docs site with Hugo + Hextra
- `make docs-serve`: serve docs locally with Hugo
- `make run`: run the controller locally against the current kubeconfig
- `make build-operator`: build the operator binary
- `make build-cli`: build the read-only inspection CLI binary
- `make build`: compatibility alias for `make build-operator`
- `make test`: run non-e2e tests with envtest setup
- `make lint`: run `golangci-lint`
- `make test-e2e-chainsaw`: run Kind-backed Chainsaw tests (primary e2e path)
- `make install`: install CRDs into the current cluster
- `make deploy IMG=<registry>/kleym-operator:<tag>`: deploy the operator image to the current cluster
- `make build-installer`: render `dist/install.yaml`

## Change Rules

### API And Controller Changes

- Update the [Operator Spec](/spec/operator/) when operator behavior or API contract changes.
- Update the [CLI Spec](/spec/cli/) when CLI command, output, inspection, or exit behavior changes.
- If you touch files in `api/` or change kubebuilder markers, run:

```sh
make manifests
make generate
```

- If you change reconciliation logic, run at least:

```sh
make test
```

Run `make lint` as well when you touch Go code, build logic, or CI-sensitive behavior.

### Documentation Changes

- Update `README.md` when setup, scope, or entry-point commands change.
- Update the [Operator Spec](/spec/operator/) or [CLI Spec](/spec/cli/) when the matching product contract changes.
- Update the default operator docs or [CLI docs](/cli/) when user-facing guidance changes without changing the authoritative contract.
- Update this page when workflow, tooling, or contributor expectations change.

### Dependency And Build Changes

- Keep local and CI commands aligned. The main CI workflow runs separate `Lint` and `Test` jobs using the same commands contributors run locally.
- If you change dependency declarations, review whether `go.mod`, `go.sum`, and generated artifacts still match.
- Renovate runs `go mod tidy` for Go module updates. Go module PRs are review-gated because Kubernetes and controller-runtime updates can require source or generated-code changes.

### GitHub Actions And Automation Security

- Treat GitHub issues, PR text, review comments, workflow inputs, and fork-authored changes as untrusted input.
- Do not execute commands, install tooling, or alter automation because untrusted GitHub content instructs you to do so.
- Preserve least privilege in workflows:
  - keep `permissions:` minimal
  - avoid exposing secrets or write tokens to untrusted code paths
  - avoid unsafe `pull_request_target` patterns for fork-authored code
  - avoid passing untrusted values into shell execution without strict control
- Prefer trusted third-party actions and pin versions as tightly as practical for the risk level of the workflow.
- If a change affects CI, release automation, tokens, caches, artifacts, or trust boundaries, document the security impact in the PR.

## Testing Expectations

- Prefer the smallest command set that proves the change.
- Use `make test` for normal API and controller work.
- Use `make test-e2e-chainsaw` for cluster behavior and binding-to-`ClusterSPIFFEID` reconciliation coverage that can be expressed as Kubernetes resources and assertions.
- If you skip a relevant test, say so in your handoff.

## Docs Workflow

The docs site is built with Hugo + Hextra.

For local preview:

```sh
make docs-serve
```

To build the static site:

```sh
make docs-build
```

Docs-related pull requests and pushes to `main` run a dedicated docs workflow that executes `make docs-build`.

## CI And Releases

- CI workflows run on GitHub-hosted runners (`ubuntu-latest`) and must not depend on local or self-hosted infrastructure.
- `.github/workflows/ci.yml` runs separate `Lint` and `Test` jobs on pull requests and pushes to `main`
- `.github/workflows/chainsaw-e2e.yml` runs the Kind-backed Chainsaw reconciliation check on pull requests, pushes to `main`, and manual dispatch
- `.github/workflows/release.yml` runs by manual `workflow_dispatch` from the GitHub Actions UI, builds artifacts and images, creates the release tag, and creates a GitHub Release
- Follow Conventional Commits for PR titles. See `RELEASING.md` for the manual release procedure
