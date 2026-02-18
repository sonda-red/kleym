# Purpose

`kleym` is a Kubernetes operator that makes inference workloads legible to workload identity by translating inference intent into deterministic SPIFFE identities. It compiles that intent into SPIRE Controller Manager resources, primarily [`ClusterSPIFFEID`][clusterspiffeid].

Scope boundary: `kleym` is an identity registration compiler. Inference deployment, traffic routing, and policy evaluation stay in the inference stack, gateway, mesh, or external policy engines.

# Core Problem

Inference stacks can be deployed reliably, but identity registration remains manual, inconsistent, and hard to standardize across teams. GAIE defines inference specific objects and clearer responsibility boundaries, but it does not define identity semantics. `kleym` bridges this gap by deriving stable SPIFFE ID templates and tenant safe selectors from GAIE resources.

# Core Value

1. Deterministic identities derived from GAIE metadata rather than ad hoc labels.
2. A single namespaced control surface for identity intent that works across heterogeneous inference stacks that share GAIE semantics.
3. Low operational risk by delegating issuance and rotation to SPIRE Controller Manager.

# Dependencies

1. SPIRE Server and SPIRE Agent.
2. SPIRE Controller Manager and its [`ClusterSPIFFEID`][clusterspiffeid] CRD.
3. `kleym` writes [`ClusterSPIFFEID`][clusterspiffeid] and does not write SPIRE entries directly.

# Preferred Inference Signal

GAIE v1 objects are the primary signal.

1. [`InferenceObjective`][gaie-inferenceobjective] is the primary model level object and references an [`InferencePool`][gaie-inferencepool] via `poolRef`.
2. [`InferencePool`][gaie-inferencepool] defines the serving pod pool for inference traffic.
3. [`InferenceModel`][gaie-inferencemodel-legacy] is treated as legacy.

# Identity Model

1. Pool identity. One SPIFFE identity representing the serving pool pods.
2. Objective identity. One SPIFFE identity per [`InferenceObjective`][gaie-inferenceobjective], representing the model endpoint at the GAIE layer even when multiple objectives share the same pool.

# Constraint

Multiple [`ClusterSPIFFEID`][clusterspiffeid] resources can select the same pod set, which can result in multiple identities applying to the same pods. Some workloads only support one SVID reliably. Clusters that require per objective identities may need to restrict or disable any default identity that would collide.

# MVP API Surface

External CRDs consumed

1. GAIE [`InferencePool`][gaie-inferencepool]
2. GAIE v1 [`InferenceObjective`][gaie-inferenceobjective]

`kleym` CRD

`InferenceTrustBinding` expresses identity intent for a single [`InferenceObjective`][gaie-inferenceobjective].

`InferenceTrustBinding` spec

1. `targetRef` references an [`InferenceObjective`][gaie-inferenceobjective] in the same namespace.
2. `spiffeIDTemplate` optionally overrides the computed template. Default is deterministic and includes trust domain, namespace, and objective name.
3. `selectorSource` is `"DerivedFromPool"`. `kleym` derives pod selection from the objective `poolRef` and validates it.
4. `workloadSelectorTemplates` are required safety constraints for selectors, at minimum namespace and service account.
5. `mode` is `"PoolOnly"` or `"PerObjective"`. Default is `"PerObjective"`.

`InferenceTrustBinding` status

1. `computedSpiffeIDs` lists identities created, including pool and objective identities.
2. `renderedSelectors` shows the final selectors applied to [`ClusterSPIFFEID`][clusterspiffeid].
3. `conditions` include `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, `RenderFailure`.

# Controller Behavior

1. Watch `InferenceTrustBinding`, [`InferenceObjective`][gaie-inferenceobjective], and [`InferencePool`][gaie-inferencepool].
2. Resolve `targetRef` to [`InferenceObjective`][gaie-inferenceobjective], then resolve `poolRef` to [`InferencePool`][gaie-inferencepool].
3. Derive pod selection from [`InferencePool`][gaie-inferencepool], then intersect it with `workloadSelectorTemplates`.
4. Reconcile one or more [`ClusterSPIFFEID`][clusterspiffeid] resources in `spire.spiffe.io` using the computed SPIFFE IDs and validated selectors.
5. Update status and emit events for conflicts, unsafe selection, and render failures.

# Multi Tenant Safety

1. `InferenceTrustBinding` is namespaced and only references objects in the same namespace.
2. Derived selectors must be proven to stay within the namespace. If they can match outside, reconciliation is refused and `UnsafeSelector` is set.
3. Ambiguous bindings where the derived selection cannot be proven to correspond to the referenced pool are refused.

# Multiple Objectives to One Pod Set

GAIE commonly maps multiple objectives to one pool. In `kleym`, the pool defines where it runs and the objective defines what it is. `kleym` can produce distinct objective identities while selectors still target the same pods.

# Acceptance Criteria

1. In a cluster with existing GAIE [`InferencePool`][gaie-inferencepool] and [`InferenceObjective`][gaie-inferenceobjective] resources, creating an `InferenceTrustBinding` creates stable [`ClusterSPIFFEID`][clusterspiffeid] resources and remains stable under resync.
2. Multiple objectives referencing one pool produce distinct SPIFFE IDs without unsafe selector expansion.
3. Overly broad or malicious selector expansion is rejected with clear status conditions.
4. `kleym` does not create or modify inference deployments, pools, routes, [`Gateway`][gateway-api-gateway], [`HTTPRoute`][gateway-api-httproute], or schedulers.

# Packaging

1. Helm chart includes `kleym` CRDs and controller deployment.
2. License is Apache 2.0.

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[gaie-inferenceobjective]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/
[gaie-inferencemodel-legacy]: https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/
[gateway-api-gateway]: https://gateway-api.sigs.k8s.io/api-types/gateway/
[gateway-api-httproute]: https://gateway-api.sigs.k8s.io/api-types/httproute/
