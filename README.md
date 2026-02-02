# terence

terence (trust inference) is a small Kubernetes add-on that attaches workload identity and trust semantics to inference backends.

## What it is
A controller that binds inference workloads to SPIFFE/SPIRE identity primitives and optional trust signals (mTLS posture, basic attribution logging). It is designed to attach to existing model deployments, including llm-d ModelService and plain Deployments.

## What it is not
- Not a gateway or API front door
- Not a request router, load balancer, or traffic shaping layer
- Not a scheduler or placement system
- Not a model serving framework, model registry, or autoscaler
- Not a replacement for llm-d, vLLM, KServe, or any inference engine

## Relationship to SPIFFE/SPIRE
terence does not reimplement SPIRE. It relies on SPIFFE/SPIRE for workload identity (SVID issuance, rotation) and uses Kubernetes-native SPIRE integration patterns (for example via SPIRE Controller Manager) to keep identities aligned with pods.

## Relationship to llm-d ModelService
llm-d ModelService owns inference deployment patterns and performance-oriented topology (disaggregation, routing integration, etc.). terence treats ModelService as a target and attaches identity/trust semantics to the workloads it produces. terence does not own llm-d routing, scheduling, or model runtime choices.

## MVP constraints
The MVP is intentionally minimal:
- One target workload (vLLM or equivalent) running on Kubernetes
- SPIFFE identity attached to the inference pods
- mTLS-only access path to the inference endpoint
- Attribution-friendly logs (who served what, when)

Non-goals for the MVP:
- Gateway API integration
- Advanced policy engines beyond identity/mTLS
- Autoscaling, multi-model orchestration, or multi-tenant UI
- Per-request / per-session identities
- Provenance receipts or confidential computing attestation (future work)
