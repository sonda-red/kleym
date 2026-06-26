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

## Operator Configuration

`kleym-operator` requires install-level identity configuration at startup.
Command-line flags are the canonical configuration surface. Environment
variables are startup-only fallbacks when the matching flag is omitted; they are
not watched or reloaded after process start.

| Flag | Environment fallback | Required | Behavior |
| --- | --- | --- | --- |
| `--trust-domain=<value>` | `KLEYM_TRUST_DOMAIN` | yes | Sets the SPIRE Server trust domain used when rendering every SPIFFE ID. The value must not include `spiffe://`, must not contain `/`, and must not include leading or trailing whitespace. |
| `--clusterspiffeid-class-name=<value>` | `KLEYM_CLUSTERSPIFFEID_CLASS_NAME` | no | Sets `spec.className` on every managed `ClusterSPIFFEID`. When empty, Kleym omits `spec.className` and keeps classless output. |

Explicit flags take precedence over environment variables, including explicit
empty values. Missing trust domain from both `--trust-domain` and
`KLEYM_TRUST_DOMAIN` fails startup with
`trustDomain must be configured before Kleym can render SPIFFE IDs`. Values
loaded from environment variables use the same validation rules as flag values.

`trustDomain` and `ClusterSPIFFEID` class are deployment concerns, not per-binding inference identity intent. They are not fields in `InferenceIdentityBinding.spec`.

When `--clusterspiffeid-class-name` is empty, SPIRE Controller Manager must be configured to watch classless `ClusterSPIFFEID` resources, for example with its `watchClassless` behavior. When a class name is set, SPIRE Controller Manager must watch that class.

## API Contract

`InferenceIdentityBinding` is namespaced. Pool and objective references stay in that namespace.

1. `poolRef` references one [`InferencePool`][gaie-inferencepool]. The pool is the required workload anchor and selector provenance source.
2. `objectiveRef` references one [`InferenceObjective`][gaie-inferenceobjective]. It is required for `PerObjective`; the objective must reference the same pool as `poolRef`.
3. `mode` is `PoolOnly` or `PerObjective`. These are the only identity boundaries. The default is `PerObjective`.
4. `serviceAccountName` is required. Kleym renders safety selectors internally as `k8s:ns:<binding namespace>` and `k8s:sa:<serviceAccountName>`.
5. SPIFFE IDs are always deterministic under the configured trust domain:
   - `PoolOnly`: `spiffe://<trustDomain>/ns/<namespace>/pool/<pool-name>`
   - `PerObjective`: `spiffe://<trustDomain>/ns/<namespace>/objective/<objective-name>`
6. `containerName` is required for `PerObjective` and must be empty for `PoolOnly`.
7. Status records `trustDomain`, `clusterSPIFFEIDClassName`, `computedSpiffeIDs`, `renderedSelectors`, and conditions. Conditions include `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, and `RenderFailure`.
8. `trustDomain` and `clusterSPIFFEIDClassName` record the operator config values used for the latest status update. They are observation data for read-only inspection compatibility; they do not make trust domain or class name per-binding spec intent.
9. The CRD exposes printer columns for `MODE`, `POOL`, `OBJECTIVE`, `READY`, `REASON`, and `SPIFFE ID` so `kubectl get inferenceidentitybindings.kleym.sonda.red -A` is the primary binding list view.

Field details live in [API Reference][api-reference]. Condition details live in [Conditions Reference][conditions-reference].

## Required Behavior

1. Discover supported GAIE pool and objective GVKs served by the cluster and watch only that subset.
2. Fail startup when no supported `InferencePool` GVK is available. Objective GVKs are optional for `PoolOnly`.
3. Resolve `poolRef` and `objectiveRef` only to documented supported GAIE groups.
4. Derive pod selection from the referenced pool, then combine it with internal namespace and service-account safety selectors and, in `PerObjective` mode, `k8s:container-name:<containerName>`.
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
