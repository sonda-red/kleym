# kleym

kleym is a small Kubernetes add-on that **attaches** workload identity and trust semantics to **existing** inference backends.

## What it is

A controller that watches for inference workloads and binds them to SPIFFE/SPIRE identity primitives and optional trust signals (mTLS posture, attribution logging). It **attaches to** existing model deployments—including llm-d ModelService, vLLM Helm charts, and plain Deployments—rather than creating them.

### Core Responsibilities

1. **Watch** for inference workloads matching `InferenceTrustBinding` selectors
2. **Issue** per-pod SPIFFE identities via SPIRE integration
3. **Configure** mTLS enforcement between clients and inference pods
4. **Emit** attribution logs with caller and workload identities

## What it is not

- Not a gateway or API front door
- Not a request router, load balancer, or traffic shaping layer
- Not a scheduler or placement system
- Not a model serving framework, model registry, or autoscaler
- Not a replacement for llm-d, vLLM, KServe, or any inference engine
- Not a SPIRE replacement—it integrates with SPIRE

## How it integrates with upstream projects

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

## Relationship to SPIFFE/SPIRE

kleym does not reimplement SPIRE. It relies on SPIFFE/SPIRE for workload identity (SVID issuance, rotation) and uses Kubernetes-native SPIRE integration patterns (e.g., SPIRE Controller Manager) to keep identities aligned with pods.

## Relationship to llm-d ModelService

llm-d ModelService owns inference deployment patterns and performance-oriented topology (disaggregation, routing integration, etc.). kleym treats ModelService as a **target** and attaches identity/trust semantics to the workloads it produces. kleym does not own llm-d routing, scheduling, or model runtime choices.

## Policy integration

Policy is **optional and pluggable**:
- `InferenceTrustBinding.spec.policyRef` references external policy resources (OPA, Gatekeeper)
- kleym does not evaluate policies itself—it passes references to policy engines
- If no `policyRef` is set, kleym still provides identity and mTLS without policy enforcement

## MVP constraints

The MVP is intentionally minimal:
- Watch inference workloads running on Kubernetes
- Attach SPIFFE identity to matched inference pods
- Configure mTLS-only access path to inference endpoints
- Emit attribution-friendly logs (who served what, when)

### Non-goals for the MVP

- Gateway API integration
- Advanced policy engines beyond identity/mTLS
- Autoscaling, multi-model orchestration, or multi-tenant UI
- Per-request / per-session identities
- Provenance receipts or confidential computing attestation (future work)
- **Creating or managing inference deployments** (use llm-d, vLLM charts, or plain Deployments)
