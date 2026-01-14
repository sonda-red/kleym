# Terence - AI Coding Guidelines

## What This Project Is

A minimal Kubernetes operator that attaches SPIFFE/SPIRE workload identity and trust semantics to inference backends (vLLM, llm-d ModelService). **Not** a gateway, router, scheduler, or model serving framework—it's identity/mTLS glue for existing inference deployments.

## Architecture

- **CRD**: `InferenceTrustProfile` in [api/v1alpha1/](../api/v1alpha1/) - currently scaffolded with placeholder `Foo` field
- **Controller**: [internal/controller/inferencetrustprofile_controller.go](../internal/controller/inferencetrustprofile_controller.go) - empty reconciler loop (TODO implementation)
- **Built with**: Kubebuilder v4.10.1, controller-runtime v0.22.4, Go 1.25
- **Domain**: `terence.sonda.red.sonda.red` (note: double `.sonda.red` - check [PROJECT](../PROJECT) file)

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
