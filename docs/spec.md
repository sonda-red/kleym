# Purpose

`kleym` is a Kubernetes operator that makes inference workloads legible to workload identity by turning inference intent into deterministic [SPIFFE](https://spiffe.io/) identities.

It does not deploy inference. It does not route inference traffic. It does not evaluate request policy. It compiles identity intent into [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager) resources.

# Core Problem

Teams can deploy inference stacks, but identity registration is still manual and inconsistent. [GAIE](https://gateway-api-inference-extension.sigs.k8s.io/) introduces inference-specific objects and separation of responsibilities, but identity is not standardized at that layer. `kleym` bridges this by deriving stable identity templates and safe selectors from GAIE resources.

# Core Value

- Deterministic model and workload identities derived from inference metadata, not ad hoc labels.
- A single, tenant-safe control surface for identity intent that works across heterogeneous inference stacks that share GAIE semantics.
- Low operational risk by delegating issuance and rotation to [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager).

# Non-Goals for the MVP

- Building a gateway, router, scheduler, or inference runtime.
- Creating `Deployment`, `Service`, `InferencePool` resources, or `HTTPRoute`s.
- Storing prompts or responses.
- Evaluating access policy inside `kleym`; external policy engines remain external.
- Guaranteeing mTLS enforcement by itself. mTLS enforcement remains the responsibility of a gateway or mesh. `kleym` only supplies the identities and inventory needed for enforcement.

# Required Dependencies

- [SPIRE](https://spiffe.io/spire/) Server plus SPIRE Agent.
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager) and its [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager) CRD. `kleym` writes `ClusterSPIFFEID`, not SPIRE entries directly.

# Preferred Inference Signal

- [Gateway API Inference Extension (GAIE)](https://gateway-api-inference-extension.sigs.k8s.io/). In GAIE v1, `InferenceObjective` replaces `InferenceModel`, and `InferenceObjective` references an `InferencePool` via `poolRef`.
- `InferencePool` defines the pod pool that serves inference traffic.

# Identity Model

- Workload pool identity: one SPIFFE identity that represents the serving pool pods.
- Model principal identity: one SPIFFE identity per `InferenceObjective`. This expresses "which model endpoint" at the GAIE layer even when objectives share the same serving pool.

# Important Constraint

Multiple identities can apply to the same pods by creating multiple [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager) resources that select the same pod set. Some workloads only reliably support one SVID at a time, so clusters may need to restrict or disable the default `ClusterSPIFFEID` where per-objective identities are required.

# MVP APIs

## External CRDs Consumed by `kleym`

- [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/) from GAIE.
- [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/) from GAIE v1. `InferenceModel` is treated as legacy and is not the primary target.

## `kleym` CRD

### `InferenceTrustBinding`

This CRD configures identity intent. It does not configure inference deployment.

### `InferenceTrustBinding` Spec Fields

- `targetRef`: reference to an `InferenceObjective` in the same namespace.
- `spiffeIDTemplate`: optional override for the computed SPIFFE ID template. Default is deterministic and includes trust domain, namespace, and objective name.
- `selectorSource`: fixed value `"DerivedFromPool"`. `kleym` derives pod selectors from the referenced objective's `poolRef` and validates the resulting selection.
- `workloadSelectorTemplates`: required constraints for SPIRE selectors, at minimum namespace and service account constraints.
- `mode`: `"PoolOnly"` or `"PerObjective"`. Default is `"PerObjective"`.

### `InferenceTrustBinding` Status Fields

- `computedSpiffeIDs`: list of identities created, including pool identity and objective identity.
- `renderedSelectors`: the final selectors that will be applied to `ClusterSPIFFEID`.
- `conditions`: `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, `RenderFailure`.

# Controller Behavior

- Watch `InferenceTrustBinding`, `InferenceObjective`, and `InferencePool`.
- Resolve `targetRef` to `InferenceObjective`, then resolve `poolRef` to `InferencePool`.
- Derive a pod selector from `InferencePool`, then intersect it with required workload selector templates for safety.
- Reconcile one or more `ClusterSPIFFEID` resources in `spire.spiffe.io`, using `spiffeIDTemplate` and derived selectors.
- Update status and emit events for conflicts, unsafe selection, and render failures.

# Multi-Tenant Safety

- `InferenceTrustBinding` is namespaced. It can only reference an `InferenceObjective` in the same namespace.
- Derived selectors must not match pods outside the namespace. If they would, `kleym` refuses and sets `UnsafeSelector`.
- `kleym` refuses ambiguous bindings where the derived selection cannot be proven to correspond to the referenced pool.

# Multiple Objectives to One Pod Set

Yes. Multiple `InferenceObjective` resources can reference the same `InferencePool`. That is the normal GAIE pattern for serving multiple objectives on one pool, often with different priority values.

In `kleym`, the principle is: pool defines "where it runs", objective defines "what it is". `kleym` can therefore produce distinct model principal identities for each objective while selectors can still target the same pods.

# Acceptance Criteria

- Given an existing GAIE setup with an `InferencePool` and one or more `InferenceObjective` resources, creating an `InferenceTrustBinding` results in the expected `ClusterSPIFFEID` resources being created and remaining stable under resync.
- Multiple objectives referencing one pool produce distinct SPIFFE IDs without unsafe selector expansion.
- A malicious or overly broad selection attempt is rejected with clear status conditions.
- `kleym` never creates or modifies inference deployments, pools, routes, or gateways.

# Packaging

- Helm chart includes `kleym` CRDs and controller deployment.
- License: Apache 2.0.
