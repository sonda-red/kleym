# terence Specification

Tenant-safe self-hosted inference on Kubernetes.

## What terence Is

terence is a Kubernetes operator that enforces tenant safety for inference workloads by compiling tenant intent into three enforceable primitives:

1. **Workload identity** — deterministic SPIFFE IDs bound to inference pods
2. **Model immutability** — digest-pinned artifact references
3. **Governed accelerator allocation** — DRA DeviceClass constraints with per-tenant quotas

## What terence Is Not

terence deliberately avoids becoming any of the following:

| Anti-pattern | Why we avoid it |
|--------------|-----------------|
| **Monolithic LLM platform installer** | Bundling gateway, scheduler, vLLM, storage, monitoring, and a kitchen sink of Helm values produces a product that competes with everyone and integrates with no one. |
| **Generic Kubernetes model-serving operator** | Rendering Deployments, Services, Routes, and PVC workflows duplicates what llm-d, GAIE, vLLM charts, KServe, and gateway providers already deliver. |
| **Service mesh identity in a new wrapper** | Another way to provision mTLS, SPIFFE IDs, and generic authz policies is already handled by SPIRE, Istio, Solo, and others. |
| **Parallel API surface** | Mirroring upstream objects like `InferencePool`, `HTTPRoute`, or vLLM configuration locks us into long-term compatibility debt for little unique value. |
| **Parameter explosion** | If it can be expressed as `values.yaml` templating, it probably does not belong in terence. |

### Explicit Non-Goals

1. Building an inference gateway or request router
2. Building an inference scheduler or token scheduler
3. Storing prompts or responses
4. Proving output correctness
5. Generic zero-trust for all workloads
6. Replacing llm-d, SPIRE, or device drivers

---

## Core Guarantees

For every terence-managed inference deployment:

| Guarantee | Description |
|-----------|-------------|
| **Tenant binding** | Inference workloads are allowed only in namespaces bound to a platform-defined tenant profile. |
| **Identity binding** | Each inference server runs under a deterministic SPIFFE ID encoding tenant and workload identity. |
| **Model immutability** | Each workload is pinned to an immutable model reference (OCI digest or ModelKit digest). |
| **Device governance** | Accelerator access is obtained only via approved DRA DeviceClasses with per-tenant quotas. |
| **Deployment auditability** | Every create, update, delete, or deny action produces an immutable audit record. |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     Platform Team                               │
│  ClusterTenantProfile    DeviceProfile                          │
└──────────────┬───────────────┬──────────────────────────────────┘
               │               │
               ▼               ▼
┌─────────────────────────────────────────────────────────────────┐
│                    terence Controller                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Tenant      │  │ Device      │  │ InferenceWorkload       │  │
│  │ Reconciler  │  │ Reconciler  │  │ Reconciler              │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                           │                                     │
│                    Validating Webhook                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Tenant Namespace                           │
│  InferenceWorkload ──► Deployment + ServiceAccount + DRA Claims │
│                        SPIFFE ID annotation                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Document Index

| Document | Description |
|----------|-------------|
| [OSS Core API Reference](spec/oss-core-api.md) | CRD specifications for `ClusterTenantProfile`, `DeviceProfile`, `InferenceWorkload`, `InferenceAuditRecord` |
| [OSS Core Controllers](spec/oss-core-controllers.md) | Reconciliation logic, admission control, llm-d adapter contract |
| [OSS Core Operations](spec/oss-core-operations.md) | Observability, security boundaries, acceptance criteria |
| [Pro Extensions](spec/pro-extensions.md) | Commercial features: request-level audit, multi-model identity, policy bundles |

---

## Dependencies

| Dependency | Requirement |
|------------|-------------|
| Kubernetes | v1.35+ recommended for DRA maturity |
| SPIRE | Optional but strongly recommended |
| DRA driver | Required for your accelerator vendor |

### Assumptions

1. A tenant maps to a Kubernetes Namespace
2. SPIFFE identities are issued by SPIRE or equivalent — terence integrates but does not issue
3. DRA is enabled with a device driver for your hardware
4. terence workloads are declared explicitly using terence CRDs

---

## Packaging

- OSS Core ships as Helm chart plus CRDs
- Apache 2.0 license for code
- Pro features live in separate binaries (see [Pro Extensions](spec/pro-extensions.md))
