# Trusted LLM Inference Operator

## What this is
This project is a Kubernetes Operator written in Go that deploys LLM inference services with secure defaults. It automates model serving plus workload identity, policy hooks, and audit friendly telemetry. The operator focuses on cluster level trust primitives rather than building a full end user gateway product.

## Problem statement
Teams increasingly run LLM inference inside Kubernetes, but security controls are often bolted on as API keys, ad hoc network rules, or external gateways that do not attribute actions to a cryptographic workload identity. This makes it harder to enforce least privilege, produce useful audit trails, and prepare for compliance demands around provenance and logging.

## Goals
1. Provide a simple custom resource that declares an inference service and reconciles it into a running deployment.
2. Default to vLLM as the serving backend, configured via explicit fields.
3. Ensure each inference workload receives a SPIFFE identity via SPIRE integration.
4. Provide a clean integration path for real time authorization decisions using OPA.
5. Emit structured logs, Kubernetes Events, and metrics that include workload identity and request attribution without leaking sensitive payloads by default.

## Non goals
1. Not a multi tenant gateway product with UI, account management, billing, or OAuth flows.
2. Not a replacement for full platforms like KServe. It can complement them, but this project stays small and security focused.
3. Not a full confidential computing or hardware attestation implementation in the initial MVP. The design should remain compatible with later additions.

## Core API surface
Primary custom resource: `LLMInferenceService`

Minimum fields should stay stable and explicit.

1. Model source
   1. Container image that runs vLLM
   2. Model reference or path that the container can load
2. Serving settings
   1. Port and protocol
   2. vLLM parameters exposed as a safe subset
3. Resources and placement
   1. CPU and memory requests and limits
   2. Optional GPU requirements
   3. Optional node selectors and tolerations
4. Security and identity
   1. SPIFFE and SPIRE integration mode
   2. Identity granularity, per pod or per replica
5. Policy integration
   1. Off by default
   2. OPA mode and endpoints when enabled
6. Observability
   1. Log detail level
   2. Metrics and traces toggle

Status should report readiness, endpoints, model version, and identity state.

## Reconciliation behavior
For each `LLMInferenceService` instance, the operator should reconcile:

1. Workload objects
   1. Deployment or StatefulSet for vLLM server pods
   2. Service for stable in cluster discovery
2. Identity wiring
   1. Pod template annotations and labels required for SPIRE integration
   2. Optional dynamic registration flow if the chosen SPIRE integration requires it
3. Policy wiring when enabled
   1. Configuration for policy checks, either via sidecar or external authorization callout
4. Observability objects
   1. ServiceMonitor or PodMonitor when Prometheus Operator is present
   2. Events for identity failures, policy failures, and readiness changes

Reconciliation must be idempotent and safe on retries.

## Identity model
Every model serving workload must have a SPIFFE identity.

Recommended default: per pod identity.

1. The operator ensures pods are eligible for SPIRE issuance.
2. Identity naming should encode at least the service name and namespace.
3. If per pod identity is used, uniqueness comes from the pod UID selectors rather than encoding volatile data in the SPIFFE ID itself.

Examples of identity patterns:

1. Service level identity
   `spiffe://<trust-domain>/ns/<namespace>/llm/<service-name>`
2. Per pod selectors provide uniqueness without changing the base ID semantics.

The operator should make the identity available to the server process via standard SPIFFE Workload API consumption patterns, typically through a socket exposed by the SPIRE agent.

## Policy model
Policy is optional, but first class.

The operator should support an authorization check that can decide allow or deny for:

1. Incoming inference requests
2. Optional tool calls or external actions if the serving stack supports it

OPA integration should accept inputs that include:

1. Caller identity, ideally a SPIFFE ID when mTLS is used
2. Target service identity or service name
3. Request metadata such as model name, route, and method
4. Optional classification signals, but default should avoid sending full prompt text

Policy outcomes should be auditable and should not be silent failures.

## Secure networking posture
The operator does not build a gateway product, but it should make secure networking easy.

1. In cluster traffic should be compatible with service mesh mTLS.
2. When mesh is present, identities should be used for workload authentication.
3. When mesh is absent, the operator can still run with SPIFFE identities for future integration, and rely on cluster network policies for basic isolation.

## Audit and provenance
The MVP should deliver useful audit trails without claiming full provenance guarantees.

1. Logs should include
   1. LLMInferenceService name and namespace
   2. Workload identity, the SPIFFE ID
   3. Caller identity when available
   4. Request id and timestamp
2. Avoid logging raw prompts and outputs by default.
3. Provide a configuration option for hashing prompt and output to support correlation without plaintext retention.

A future extension can add signed inference metadata records, but it should remain clearly optional.

## Hardware aware scheduling
The operator should use Kubernetes native scheduling primitives.

1. GPU usage should be expressed as resource requests and limits.
2. Node selection should rely on node selectors, affinities, and tolerations.
3. The operator should not implement a custom scheduler.

Intel acceleration support is a configuration concern, not an architectural one. The operator should allow selecting relevant container images or runtime flags that enable OpenVINO or IPEX paths where applicable.

## Incremental milestones
Milestone 1, MVP
1. `LLMInferenceService` CRD
2. Reconcile to Deployment plus Service
3. vLLM container with basic config
4. SPIRE integration via pod annotations and selectors
5. Basic readiness and status reporting

Milestone 2, identity quality and observability
1. Per pod identity as default
2. Structured logs with SPIFFE IDs
3. Metrics for request counts and error rates

Milestone 3, policy integration
1. Optional OPA integration for allow or deny
2. Audit of policy decisions

Milestone 4, advanced trust features
1. Node attestation aware modes, tied to SPIRE selectors
2. Optional signed inference metadata records
3. Confidential container compatibility hooks when relevant

## Differentiation
This project is not trying to be a general model serving platform. Its niche is the intersection of:

1. Kubernetes native automation for LLM serving
2. Workload identity via SPIFFE and SPIRE as a first class primitive
3. Real time policy enforcement hooks that can be audited
4. A design that stays small enough for a solo operator project and still demonstrates end to end security ownership

## Success criteria
1. A user can apply a single CR and get a working inference endpoint in cluster.
2. The deployed pods receive a SPIFFE identity reliably.
3. Logs and events provide identity tagged evidence of what is running and who called it.
4. Policy checks can be enabled without rewriting the serving stack.
5. The codebase remains simple, testable, and easy to extend.
