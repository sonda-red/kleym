# kleym - AI Coding Guidelines

## What This Project Is

A minimal Kubernetes operator that **attaches** SPIFFE/SPIRE workload identity and trust semantics to **existing** inference backends (vLLM, llm-d ModelService, plain Deployments). **Not** a gateway, router, scheduler, or model serving framework—it's identity/mTLS glue that watches inference workloads and binds them to trust primitives.

## Purpose

kleym operates on the **governance plane**: it does not deploy, scale, or configure inference runtimes. It watches for inference workloads created by other tools (llm-d, Helm charts, GitOps, etc.) and attaches identity, mTLS enforcement, and audit-grade logging.

### Primary Goals

1. **Watch for inference workloads** matching label selectors defined in `InferenceTrustBinding` resources
2. **Issue per-pod SPIFFE identities** via SPIRE or compatible integrations (SPIRE Controller Manager, Kubernetes Workload Registrar)
3. **Configure mTLS enforcement** between clients and inference pods using SPIRE-issued SVIDs
4. **Emit attribution logs** that include caller identity, workload SPIFFE ID, and request metadata

### What the CRD Does

`InferenceTrustBinding` configures **trust behaviour**, not inference deployment:
- `selector`: Which workloads to attach identity to
- `spiffeIdScope`: Granularity of identity (pod, replicaSet, deployment)
- `mtlsRequired`: Whether to enforce mTLS
- `policyRef`: Optional reference to external policy (OPA, Gatekeeper)
- `attributionLog`: Audit log format and content

The CRD contains **no fields** for images, replicas, resources, models, or runtime configuration—those belong to the inference deployment owned by upstream tools.

### Explicit Non-Goals

1. **Do not build** an inference gateway, request router, or API front door
2. **Do not build** an inference scheduler or token scheduler
3. **Do not create** Deployments, StatefulSets, or Services for inference workloads—attach to existing ones
4. **Do not reimplement** vLLM, llm-d, KServe, or any model runtime
5. **Do not replace** SPIRE—integrate with it
6. **Do not implement** policy engines—reference external OPA/Gatekeeper policies
7. **Do not add** autoscaling, multi-tenancy UI, or model registry features

### Future Considerations (Not MVP)

- Per-request / per-session identities
- Provenance receipts or confidential computing attestation
- Gateway API integration
- Advanced policy engines beyond identity/mTLS

## Architecture

- **CRD**: `InferenceTrustBinding` in [api/v1alpha1/](../api/v1alpha1/) - configures identity scope, mTLS, policy refs, attribution logging
- **Controller**: [internal/controller/inferencetrustbinding_controller.go](../internal/controller/inferencetrustbinding_controller.go) - watches workloads, issues identities, enforces mTLS
- **Built with**: Kubebuilder v4.10.1, controller-runtime v0.22.4, Go 1.25
- **Domain**: `kleym.sonda.red`

## How kleym Integrates with Upstream Projects

```
┌─────────────────────────────────────────────────────────────────┐
│  Upstream Deployment Tools (NOT owned by kleym)               │
│  llm-d ModelService │ vLLM Helm │ Plain Deployments │ KServe    │
└──────────────────────────────┬──────────────────────────────────┘
                               │ creates inference Pods
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                    kleym Controller                           │
│  1. Watches Pods matching InferenceTrustBinding.selector        │
│  2. Requests SPIFFE ID from SPIRE for each matched Pod          │
│  3. Injects mTLS sidecar or configures native SPIFFE support    │
│  4. Emits attribution logs with identity metadata               │
└──────────────────────────────┬──────────────────────────────────┘
                               │ annotates / patches
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  SPIRE (NOT owned by kleym)                                   │
│  Issues SVIDs, rotates certificates, provides trust bundles     │
└─────────────────────────────────────────────────────────────────┘
```

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
make test-e2e    # Creates Kind cluster "kleym-test-e2e", runs e2e, tears down
make lint        # golangci-lint v2.5.0
```

### Build & Deploy
```bash
make docker-build IMG=your-registry/kleym:tag
make deploy IMG=your-registry/kleym:tag  # Uses kustomize
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
- **Never create inference workloads**—only watch, annotate, and configure identity

## MVP Scope Constraints

**IN SCOPE**: SPIFFE identity attachment, mTLS enforcement, attribution logs for inference workloads  
**OUT OF SCOPE**: Gateway API, policy engines beyond identity/mTLS, autoscaling, multi-tenancy, per-request identities, provenance/attestation, deployment creation

⚠️ **Warning**: Do not add deployment logic or runtime coupling unless there is no upstream solution. kleym attaches to workloads; it does not create them.

## Policy Integration

Policy is **optional and pluggable**:
- `InferenceTrustBinding.spec.policyRef` references external policy resources
- kleym does not evaluate policies—it passes references to OPA/Gatekeeper
- Policy decisions should be auditable and traceable to SPIFFE identities
- If no `policyRef` is set, kleym still provides identity and mTLS without policy enforcement

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

**Add new field to CRD**: Edit `api/v1alpha1/inferencetrustbinding_types.go` → `make manifests generate` → check `config/crd/bases/`

**Add RBAC permission**: Add `// +kubebuilder:rbac` marker to controller → `make manifests` → check `config/rbac/role.yaml`

**Run locally**: `make install` (installs CRDs) → `make run` (runs controller out-of-cluster)

**View logs**: Controller uses controller-runtime's structured logging (`logr`). Get logger with `logf.FromContext(ctx)`.

