---
title: Operator Spec
weight: 10
---

`kleym-operator` is an identity registration compiler. It translates inference intent into deterministic Secure Production Identity Framework for Everyone (SPIFFE) identities and writes SPIFFE Runtime Environment (SPIRE) Controller Manager [`ClusterSPIFFEID`][clusterspiffeid] resources.

Kleym stops at identity registration. It does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or write SPIRE registration entries directly.

## Scope

Kleym owns `InferenceIdentityBinding`, GAIE input resolution, SPIFFE ID rendering, selector safety, managed `ClusterSPIFFEID` reconciliation, status, events, and finalizer cleanup.

Kleym does not own inference workloads, schedulers, routes, gateways, serving behavior, Envoy, OPA, OAuth, OIDC, SPIRE Server, SPIRE Agent, credential issuance, authorization, or audit decisions.

Dependency facts live in [Dependencies][dependencies]. Supported GAIE inputs live in [GAIE Compatibility][gaie-compatibility].

## API Contract

`InferenceIdentityBinding` is namespaced. Pool and objective references stay in that namespace.

1. `poolRef` references one [`InferencePool`][gaie-inferencepool]. The pool is the required workload anchor and selector provenance source.
2. `objectiveRef` references one [`InferenceObjective`][gaie-inferenceobjective]. It is required for `PerObjective`; the objective must reference the same pool as `poolRef`.
3. `mode` is `PoolOnly` or `PerObjective`. These are the only identity boundaries. The default is `PerObjective`.
4. `spiffeIDTemplate` may override the computed SPIFFE ID. Defaults are deterministic:
   - `PoolOnly`: `spiffe://kleym.sonda.red/ns/<namespace>/pool/<pool-name>`
   - `PerObjective`: `spiffe://kleym.sonda.red/ns/<namespace>/objective/<objective-name>`
5. `selectorSource` is `DerivedFromPool`.
6. `workloadSelectorTemplates` are required safety constraints. Rendered selectors must include `k8s:ns:<namespace>` and `k8s:sa:<service-account>`.
7. `containerDiscriminator` is required for `PerObjective` and must be empty for `PoolOnly`.
8. Status records `computedSpiffeIDs`, `renderedSelectors`, and conditions. Conditions include `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, and `RenderFailure`.

Field details live in [API Reference][api-reference]. Condition details live in [Conditions Reference][conditions-reference].

## Required Behavior

1. Discover supported GAIE pool and objective GVKs served by the cluster and watch only that subset.
2. Fail startup when no supported `InferencePool` GVK is available. Objective GVKs are optional for `PoolOnly`.
3. Resolve `poolRef` and `objectiveRef` only to documented supported GAIE groups.
4. Derive pod selection from the referenced pool, then intersect it with rendered safety selectors and, in `PerObjective` mode, the container discriminator.
5. Refuse unsafe selectors. If the selector set cannot be proven to stay within the binding namespace and required service account boundary, set `UnsafeSelector` and produce no managed output.
6. Render the SPIFFE ID and managed `ClusterSPIFFEID` shape deterministically. Rendered output fields are documented in [Managed Resources][managed-resources].
7. Refuse identity collisions. If two `PerObjective` bindings would match the same pod set and same container-name value, set `Conflict=True` with reason `IdentityCollision` on both resources and reconcile neither until fixed.
8. Treat missing required CRDs and infrastructure-not-ready states as transient by retrying reconciliation on a timer.
9. On deletion, delete managed `ClusterSPIFFEID` children first and keep the binding finalizer until a follow-up list confirms no managed children remain.

Collision behavior is expanded in [Collision Detection][collision-detection]. Selector rationale is expanded in [Selector Safety][selector-safety].

## Safety Invariants

1. `InferenceIdentityBinding` is namespaced.
2. Pool and objective references stay in the binding namespace.
3. `PoolOnly` and `PerObjective` are the only identity boundaries.
4. Unsafe selectors are refused.
5. Identity collisions are refused with `Conflict` reason `IdentityCollision`.
6. Deletion keeps the finalizer until managed children are gone.
7. `kleym-operator` does not create or modify inference deployments, pools, routes, gateways, schedulers, or policy resources.

## References

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[gaie-inferenceobjective]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/
[api-reference]: /reference/api/
[conditions-reference]: /reference/conditions/
[dependencies]: /reference/dependencies/
[gaie-compatibility]: /reference/gaie-compatibility/
[managed-resources]: /reference/resources/
[collision-detection]: /design/collision-detection/
[selector-safety]: /design/selector-safety/
