# Terence - AI Coding Guidelines

## What This Project Is

A minimal Kubernetes operator that attaches SPIFFE/SPIRE workload identity and trust semantics to inference backends (vLLM, llm-d ModelService). **Not** a gateway, router, scheduler, or model serving framework—it's identity/mTLS glue for existing inference deployments.

## Purpose

# Copilot instructions for this repository

Purpose
Build a Kubernetes Operator in Go that deploys LLM inference services with strong identity, policy hooks, and auditable telemetry. The operator should automate a secure default deployment on Kubernetes.

Primary goals
1. Reconcile a custom resource named LLMInferenceService into a working inference Deployment or StatefulSet.
2. Use vLLM as the default serving engine.
3. Integrate SPIFFE and SPIRE so each inference workload gets a SPIFFE ID suitable for mTLS and audit attribution.
4. Make policy enforcement pluggable, with a first class integration path for OPA.
5. Produce audit friendly logs that include caller identity and model workload identity.

Non goals
1. Do not build a full multi tenant gateway product, UI, or account management system.
2. Do not reimplement model runtimes. Prefer configuration of vLLM and existing ecosystem components.
3. Do not implement confidential computing or hardware attestation in the initial MVP. Keep design compatible with later additions.

Operator scope and conventions
1. Use Kubebuilder and controller runtime conventions.
2. Keep CRDs minimal and stable. Prefer explicit fields over clever inference.
3. Prefer reconciliation that is idempotent and safe on retries.
4. Prefer small, reviewable PR sized changes with clear tests.

Identity requirements
1. Every model workload must have a SPIFFE identity.
2. Prefer per replica or per pod identity rather than one identity shared by all replicas.
3. Prefer integration via SPIRE Kubernetes mechanisms when possible. If direct registration API use is needed, keep it optional and well isolated.

Policy requirements
1. Policy is optional but supported as a module.
2. Provide an integration path for OPA that can evaluate allow or deny decisions for requests or tool calls.
3. Policy decisions should be auditable and traceable to identities.

Hardware and performance constraints
1. Workloads may target CPU, GPU, or Intel acceleration paths. Keep configuration explicit.
2. Prefer Kubernetes native scheduling, resource requests, and node selection. Do not invent a scheduler.

Logging and observability
1. Log request context without storing sensitive payloads by default.
2. Include SPIFFE IDs and workload identifiers in logs and events.
3. Prefer metrics that help operators understand throughput, errors, and scheduling.

When implementing
1. Do not add features outside the MVP without an explicit issue or task.
2. If requirements are unclear, propose a minimal implementation and list the assumptions inside the PR description or a design note.

## Architecture

- **CRD**: `InferenceTrustProfile` in [api/v1alpha1/](../api/v1alpha1/) - currently scaffolded with placeholder `Foo` field
- **Controller**: [internal/controller/inferencetrustprofile_controller.go](../internal/controller/inferencetrustprofile_controller.go) - empty reconciler loop (TODO implementation)
- **Built with**: Kubebuilder v4.10.1, controller-runtime v0.22.4, Go 1.25
- **Domain**: `terence.sonda.red`

## Critical Development Workflow

### Code Generation (Required After API Changes)
```bash
make manifests  # Regenerates CRDs, RBAC from markers
make generate   # Regenerates DeepCopy methods
```
Always run both after editing types in `api/v1alpha1/*_types.go`.

### Testing
```bash
make test        # Unit tests with envtest (K8s 1.35 API server)
make test-e2e    # Creates Kind cluster "terence-test-e2e", runs e2e, tears down
make lint        # golangci-lint v2.5.0
```

### Build & Deploy
```bash
make docker-build IMG=your-registry/terence:tag
make deploy IMG=your-registry/terence:tag  # Uses kustomize
make undeploy    # Removes all resources
```

## Project Conventions

### Kubebuilder Markers
Use standard markers in Go files (see existing examples):
- `// +kubebuilder:rbac:` for RBAC generation
- `// +optional`, `// +required` for CRD validation
- `// +kubebuilder:subresource:status` for status subresource

### GOFLAGS Quirk
`export GOFLAGS := -buildvcs=false` is set in Makefile - VCS stamping disabled project-wide.

### Controller Pattern
- Reconcile function should be idempotent
- Use `ctrl.Result{}` for success, `ctrl.Result{Requeue: true}` or `ctrl.Result{RequeueAfter: duration}` for retries
- Status updates are separate from spec reconciliation (use subresource client)

## MVP Scope Constraints

**IN SCOPE**: SPIFFE identity attachment, mTLS enforcement, attribution logs for inference workloads  
**OUT OF SCOPE** (per [README.md](../README.md)): Gateway API, policy engines beyond identity/mTLS, autoscaling, multi-tenancy, per-request identities, provenance/attestation

Don't add features outside MVP scope without explicit discussion.

## Release Process

**Semantic versioning** via GitHub Actions on merge to `main`. Use [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` → minor bump
- `fix:`, `perf:`, `refactor:` → patch bump  
- `BREAKING CHANGE:` footer → major bump
- `docs:`, `test:`, `ci:`, `chore:` → no release

See [SEMANTIC_VERSIONING.md](../SEMANTIC_VERSIONING.md) for details.

## Key Files to Know

- **[PROJECT](../PROJECT)**: Kubebuilder metadata - don't edit manually
- **[config/](../config/)**: Kustomize manifests for deployment
  - `config/crd/bases/` - generated CRD YAML
  - `config/manager/` - controller Deployment
  - `config/rbac/` - generated RBAC rules
  - `config/samples/` - example CR
- **[Dockerfile](../Dockerfile)**: Multi-stage build with distroless base
- **[hack/boilerplate.go.txt](../hack/boilerplate.go.txt)**: License header template for generated code

## Common Tasks

**Add new field to CRD**: Edit `api/v1alpha1/inferencetrustprofile_types.go` → `make manifests generate` → check `config/crd/bases/`

**Add RBAC permission**: Add `// +kubebuilder:rbac` marker to controller → `make manifests` → check `config/rbac/role.yaml`

**Run locally**: `make install` (installs CRDs) → `make run` (runs controller out-of-cluster)

**View logs**: Controller uses controller-runtime's structured logging (`logr`). Get logger with `logf.FromContext(ctx)`.
