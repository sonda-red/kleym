# kleym Specification

Tenant-safe self-hosted inference on Kubernetes.

## What kleym Is

kleym is a Kubernetes operator that **attaches** SPIFFE/SPIRE workload identity and trust semantics to **existing** inference workloads. It operates on the governance plane:

1. **Workload identity** — deterministic SPIFFE IDs bound to inference pods
2. **mTLS enforcement** — mutual TLS between clients and inference endpoints
3. **Attribution logging** — audit-grade logs with caller and workload identities

kleym does **not** create inference deployments. It watches for workloads created by llm-d, vLLM charts, KServe, or plain Deployments and attaches identity/trust primitives.

## What kleym Is Not

kleym deliberately avoids becoming any of the following:

| Anti-pattern | Why we avoid it |
|--------------|-----------------|
| **Monolithic LLM platform installer** | Bundling gateway, scheduler, vLLM, storage, monitoring, and a kitchen sink of Helm values produces a product that competes with everyone and integrates with no one. |
| **Generic Kubernetes model-serving operator** | Rendering Deployments, Services, Routes, and PVC workflows duplicates what llm-d, GAIE, vLLM charts, KServe, and gateway providers already deliver. |
| **Service mesh identity in a new wrapper** | Another way to provision mTLS, SPIFFE IDs, and generic authz policies is already handled by SPIRE, Istio, Solo, and others. |
| **Parallel API surface** | Mirroring upstream objects like `InferencePool`, `HTTPRoute`, or vLLM configuration locks us into long-term compatibility debt for little unique value. |
| **Parameter explosion** | If it can be expressed as `values.yaml` templating, it probably does not belong in kleym. |

### Explicit Non-Goals

1. Building an inference gateway or request router
2. Building an inference scheduler or token scheduler
3. Creating Deployments, StatefulSets, or Services for inference workloads
4. Storing prompts or responses
5. Proving output correctness
6. Replacing llm-d, SPIRE, KServe, or device drivers

### Future Considerations (Not MVP)

- Per-request / per-session identities
- Provenance receipts or confidential computing attestation
- Gateway API integration
- Advanced policy engines beyond identity/mTLS
- Tenant profiles and device governance (see [spec/oss-core-api.md](spec/oss-core-api.md) for future direction)

---

## Core Guarantees (MVP)

For every kleym-managed inference workload:

| Guarantee | Description |
|-----------|-------------|
| **Identity binding** | Each inference pod runs under a deterministic SPIFFE ID. |
| **mTLS enforcement** | Mutual TLS is configured for traffic to matched workloads. |
| **Attribution logging** | Logs include caller identity, workload SPIFFE ID, and request metadata. |

---

## Architecture Overview

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
│  3. Configures mTLS (sidecar or native SPIFFE support)          │
│  4. Emits attribution logs with identity metadata               │
└──────────────────────────────┬──────────────────────────────────┘
                               │ annotates / patches
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  SPIRE (NOT owned by kleym)                                   │
│  Issues SVIDs, rotates certificates, provides trust bundles     │
└─────────────────────────────────────────────────────────────────┘
```

---

## MVP CRD: InferenceTrustBinding

The single CRD for the MVP configures **trust behaviour**, not inference deployment:

| Field | Description |
|-------|-------------|
| `selector` | Label selector identifying which workloads to attach identity to |
| `spiffeIdScope` | Identity granularity: `pod`, `replicaSet`, or `deployment` |
| `mtlsRequired` | Whether to enforce mutual TLS |
| `policyRef` | Optional reference to external policy (OPA, Gatekeeper) |
| `attributionLog` | Audit log format and content |

The CRD contains **no fields** for images, replicas, resources, models, or runtime configuration.

See [api/v1alpha1/inferencetrustbinding_types.go](../api/v1alpha1/inferencetrustbinding_types.go) for the full type definition.

---

## Policy Integration

Policy is **optional and pluggable**:
- `policyRef` references external policy resources (OPA ConfigMap, Gatekeeper policies)
- kleym does not evaluate policies—it passes references to policy engines
- If no `policyRef` is set, kleym still provides identity and mTLS without policy enforcement

---

## Document Index

| Document | Description |
|----------|-------------|
| [OSS Core API Reference](spec/oss-core-api.md) | Future CRDs for tenant profiles, device governance, audit records |
| [OSS Core Controllers](spec/oss-core-controllers.md) | Future reconciliation logic and admission control |
| [OSS Core Operations](spec/oss-core-operations.md) | Observability, security boundaries, acceptance criteria |
| [Pro Extensions](spec/pro-extensions.md) | Commercial features: request-level audit, multi-model identity, policy bundles |

---

## Dependencies

| Dependency | Requirement |
|------------|-------------|
| Kubernetes | v1.35+ recommended |
| SPIRE | Optional but strongly recommended |

### Assumptions

1. SPIFFE identities are issued by SPIRE or equivalent — kleym integrates but does not issue
2. Inference workloads are created by upstream tools (llm-d, vLLM, etc.)
3. kleym watches and attaches to workloads; it does not create them

---

## Packaging

- Ships as Helm chart plus CRDs
- Apache 2.0 license for code
- Pro features live in separate binaries (see [Pro Extensions](spec/pro-extensions.md))
