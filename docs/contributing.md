---
title: Contributing
weight: 110
---

`kleym` is a Kubernetes operator that compiles inference identity intent into SPIRE Controller Manager resources. The repo is still early, so contributors should prefer small, explicit changes that keep the spec, code, and generated artifacts aligned.

## Sources Of Truth

- [`spec.md`](spec) is the authoritative product and API behavior document.
- `README.md` is the project entry point and quickstart.
- `AGENTS.md` is the minimal repository contract for coding agents.
- `RELEASING.md` describes the tag-based release procedure.

If code and docs disagree, fix the disagreement instead of silently choosing one.

## Check GitHub Context First

Issues and pull requests are part of the design history for this repository. Check the directly relevant GitHub context when:

- a task references an issue, PR, review thread, release note, or regression
- intended behavior is unclear from the current code and the [spec](spec)
- you are changing API contracts, reconciliation semantics, CI, release flow, or project policy

Keep that review narrow. Read the issue or PR that motivated the change and any directly related follow-ups. Avoid broad history searches unless the task genuinely requires it.

## Ticket Workflow

- If work is tied to an issue, follow the issue instructions explicitly.
- Keep scope tight. If adjacent cleanup looks useful but is not required, propose a follow-up ticket instead of bundling it in.
- Start the work on a dedicated branch, not on `main`.
- Open a PR for the branch and include an automatic issue-closing reference when an issue number exists, for example `Fixes #123` or `Closes #123`.
- Use the PR description to separate delivered scope from follow-up ideas.

## Development Prerequisites

- Go `1.25+`
- Docker
- `kubectl`
- Access to a Kubernetes cluster for deployment testing
- `kind` for local e2e testing
- Hugo Extended `0.146+` for docs preview/build

The repository bootstraps local tool binaries under `bin/` through `make` targets, so global installs of `controller-gen`, `kustomize`, `setup-envtest`, and `golangci-lint` are not required.

## Repository Map

- `cmd/main.go`: controller manager entry point
- `api/v1alpha1`: API types and generated deepcopy code for `InferenceIdentityBinding`
- `internal/controller`: reconciliation logic and controller-focused tests
- `config/`: CRD, RBAC, manager deployment, samples, and kustomize overlays
- `test/e2e`: Kind-backed end-to-end coverage
- `.github/workflows`: CI, release, and maintenance automation

## Common Commands

- `make help`: list supported targets
- `make docs-build`: build the docs site with Hugo + Hextra
- `make docs-serve`: serve docs locally with Hugo
- `make docs-build-versioned`: build root docs and configured `/versions/*` snapshots
- `make run`: run the controller locally against the current kubeconfig
- `make build`: build the manager binary
- `make test`: run non-e2e tests with envtest setup
- `make lint`: run `golangci-lint`
- `make test-e2e`: run Kind-backed end-to-end tests
- `make install`: install CRDs into the current cluster
- `make deploy IMG=<registry>/kleym:<tag>`: deploy the controller image to the current cluster
- `make build-installer`: render `dist/install.yaml`

## Change Rules

### API And Controller Changes

- Update the [spec](spec) when behavior or API contract changes.
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
- Update the [spec](spec) when the product contract changes.
- Update this page when workflow, tooling, or contributor expectations change.

### Dependency And Build Changes

- Keep local and CI commands aligned. The main CI workflow runs separate `Lint` and `Test` jobs using the same commands contributors run locally.
- If you change dependency declarations, review whether `go.mod`, `go.sum`, and generated artifacts still match.

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
- Use `make test-e2e` for cluster behavior, Kind setup, deployment flows, or bugs that only reproduce against a live control plane.
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

To build versioned output paths configured in `.docs-versions`:

```sh
make docs-build-versioned
```

Docs-related pull requests and pushes to `main` run a dedicated docs workflow that executes `make docs-build`.

## CI And Releases

- CI workflows run on GitHub-hosted runners (`ubuntu-latest`) and must not depend on local or self-hosted infrastructure.
- `.github/workflows/ci.yml` runs separate `Lint` and `Test` jobs on pull requests and pushes to `main`
- `.github/workflows/release.yml` runs on `v*` tag pushes, verifies the tag is on `main`, builds artifacts and images, and creates a GitHub Release
- Follow Conventional Commits for PR titles. See `RELEASING.md` for the tag-based release procedure
