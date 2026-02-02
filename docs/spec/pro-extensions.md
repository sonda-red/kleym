# kleym Pro Extensions

Commercial extensions for identity depth, compliance, and multi-tenant inference governance.

## Positioning

kleym Pro extends OSS Core with features that platform teams buy:
- Per-request attribution
- Compliance-grade audit and retention
- Multi-model identity semantics
- Operational governance at scale

**Pro must not be required to satisfy the core tenant safety guarantees. Pro enhances them.**

---

## Commercial Boundary

| Principle | Rule |
|-----------|------|
| OSS independence | OSS Core remains fully functional for tenant-safe deployment admission, DRA governance, and workload identity binding |
| Additive only | Pro adds deeper guarantees, never removes or modifies core behavior |
| Separate deployment | Pro deploys as an additional controller manager and webhook set, not a fork |
| Graceful degradation | Pro features disable cleanly when license is absent, OSS continues |

---

## Pro Guarantees

| Guarantee | Description |
|-----------|-------------|
| **Per-request attribution** | Every request attributed to caller identity, server identity, and model identity with verifiable records |
| **In-pod multi-model identity** | Each model instance has a unique verifiable identity bound to parent workload + model artifact |
| **Compliance-grade audit** | Tamper-evident, exportable records with retention policies, deletion workflows, legal hold |
| **Policy bundle governance** | Versioned, signed policy with approval workflows and break-glass controls |
| **Chargeback reporting** | Tenant-level accelerator usage and concurrency for billing and capacity planning |

---

## Architecture Components

```
┌─────────────────────────────────────────────────────────────┐
│                     Pro Controller Manager                   │
│  ┌──────────────────┐  ┌──────────────────────────────────┐ │
│  │ PolicyBundle     │  │ ModelInstanceIdentity            │ │
│  │ Reconciler       │  │ Reconciler                       │ │
│  └──────────────────┘  └──────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Pro Admission Webhook                      │
│  Policy bundle injection, additional validation              │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Auditor & Exporter                         │
│  Request-level events → Kafka/S3/Elastic/ClickHouse/OTEL    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│              Optional Sidecar / Node Agent                   │
│  Crypto signing, token brokering, request accounting         │
└─────────────────────────────────────────────────────────────┘
```

---

## Pro APIs

### PolicyBundle (cluster-scoped)

| Field | Type | Description |
|-------|------|-------------|
| `policyType` | string | OPA Rego, Cedar, or vendor-neutral format |
| `bundleVersion` | string | Semantic version |
| `signature` | string | Cryptographic signature |
| `rolloutStrategy` | RolloutSpec | Gradual rollout configuration |
| `targets` | []string | Tenant profiles or namespaces |

### RequestAuditSink (cluster-scoped)

| Field | Type | Description |
|-------|------|-------------|
| `backend` | string | `kafka`, `s3`, `elastic`, `clickhouse`, `otel-logs` |
| `retentionDays` | int | Retention period |
| `encryptionMode` | string | Encryption configuration |
| `piiHandling` | PIISpec | Hashing and redaction rules |

### ModelInstanceIdentity (namespaced)

Created by kleym Pro, represents a model identity inside a workload.

| Field | Type | Description |
|-------|------|-------------|
| `parentInferenceWorkloadRef` | ObjectRef | Parent workload |
| `modelDigest` | string | Model artifact digest |
| `modelInstanceId` | string | Stable identifier |
| `identityTokenIssuer` | string | `local` or external issuer |
| `tokenRotationPeriod` | Duration | Token rotation interval |

---

## Multi-Model Identity Design

**Goal:** Support multiple models per pod without pretending Kubernetes can attest each model process natively.

### Mechanism

1. Parent identity is the SPIFFE ID of the workload
2. Pro issues a signed model identity token per model instance
3. Token includes: `modelDigest`, `modelInstanceId`, `tenant`, `workload spiffeId`, `policy bundle hash`
4. Token is rotated and included in responses, logs, and audit events
5. Verifiers can validate token offline using published keys

This yields per-model identity semantics while staying compatible with SPIRE's workload attestation model.

---

## Request-Level Audit Pipeline

### Inputs
- Request metadata: trace ID, caller identity, server identity, model identity token
- Timing and resource usage metadata (optional)

### Storage
- External sink via `RequestAuditSink`
- Tamper evidence through signing and hash chaining

### Privacy Controls

| Level | Behavior |
|-------|----------|
| Default | No prompt, no response stored |
| Hashed | Store hashed prompt and response |
| Structured | Store metadata extracted by deterministic rules (never raw content unless explicitly enabled) |

---

## Integration Patterns

Pro integrates rather than replaces:

| Integration | Purpose |
|-------------|---------|
| Gateway API inference implementations | Request authorization |
| Service mesh mTLS | Caller verification |
| llm-d | Model instance identity and policy bundle propagation |

---

## Enterprise Management

- **Multi-cluster management** — Profiles and policies across clusters with drift detection
- **Delegated admin** — Role-based management of tenant and device profiles
- **Reporting** — Utilization, claim usage, concurrency, violations
- **Supportability** — Diagnostics bundles, upgrade safety checks

---

## Licensing & Delivery

| Aspect | Implementation |
|--------|----------------|
| Images | Separate Pro controller and webhook images |
| Verification | Kubernetes Secret + periodic renewal |
| Degradation | Pro features disable, OSS continues |

### Open Core Hygiene

1. Core CRDs must not depend on Pro CRDs
2. Core controller ignores Pro-only annotations if Pro is absent
3. Pro extends via annotations and optional fields, never rewrites core semantics

---

## Roadmap

### Pro 1
- RequestAuditSink and export
- Tamper-evident audit chain
- Basic policy bundle attachment

### Pro 2
- ModelInstanceIdentity and signed model tokens
- Response-level identity propagation
- Per-tenant chargeback reporting

### Pro 3
- Multi-cluster governance
- Approval workflows and break-glass
- SIEM and compliance packs

---

## Acceptance Criteria

1. ✅ Every request mappable to tenant, caller SPIFFE ID, server SPIFFE ID, and modelDigest
2. ✅ Multi-model workloads produce distinct, verifiable model instance identities
3. ✅ Audit storage meets retention/export requirements without storing raw prompts by default
4. ✅ Policy bundles are signed, versioned, and enforced consistently
5. ✅ Pro can be removed and core tenant safety still functions
