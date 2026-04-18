---
title: Spec
weight: 80
---

`kleym` is a Kubernetes operator that makes inference workloads legible to workload identity by translating inference intent into deterministic Secure Production Identity Framework for Everyone (SPIFFE) identities. It compiles that intent into SPIFFE Runtime Environment (SPIRE) Controller Manager resources, primarily [`ClusterSPIFFEID`][clusterspiffeid].

Scope boundary: `kleym` is an identity registration compiler. It stops at identity registration and selector provenance. Inference deployment, traffic routing, and policy evaluation stay in the inference stack, gateway, mesh, or external policy engines. `kleym` does not configure Envoy, Envoy Gateway, kgateway, ext authz, ext proc, OPA, Cedar, OAuth, or OIDC.

Reference docs: [SPIFFE overview][spiffe-overview], [SPIRE concepts][spire-concepts], and [SPIRE Controller Manager][spire-controller-manager].

## Core Problem

Inference stacks can be deployed reliably, but identity registration remains manual, inconsistent, and hard to standardize across teams. Gateway API Inference Extension (GAIE) defines inference-specific objects and clearer responsibility boundaries, but it does not define identity semantics. `kleym` bridges this gap by deriving stable SPIFFE ID templates and tenant safe selectors from GAIE resources.

## Core Value

1. Deterministic identities derived from GAIE metadata rather than ad hoc labels.
2. A single namespaced control surface for identity intent that works across heterogeneous inference stacks that share GAIE semantics.
3. Low operational risk by delegating issuance and rotation to SPIRE Controller Manager.

## Dependencies

1. SPIRE Server and SPIRE Agent.
2. SPIRE Controller Manager and its [`ClusterSPIFFEID`][clusterspiffeid] CRD.
3. `kleym` writes [`ClusterSPIFFEID`][clusterspiffeid] and does not write SPIRE entries directly.

## Supported Downstream Pattern

1. SPIRE issues X.509 SVIDs and/or JWT SVIDs.
2. Envoy consumes identity through SDS or through components adjacent to the SPIRE Workload API.
3. External auth policy maps SPIFFE ID to route or model authorization.
4. Audit logging happens at the gateway or policy layer, not in `kleym`.

## Preferred Inference Signal

GAIE objects are the primary signal.

1. [`InferenceObjective`][gaie-inferenceobjective] is the primary model level object and references an [`InferencePool`][gaie-inferencepool] via `poolRef`.
2. [`InferencePool`][gaie-inferencepool] defines the serving pod pool for inference traffic.
3. [`InferenceModel`][gaie-inferencemodel-legacy] is treated as legacy.
4. At startup, `kleym` discovers which supported GAIE GVKs are served by the cluster and watches only that subset.
   - `GVK` means `GroupVersionKind` (`<api-group>/<version>, Kind=<kind>`), for example:
     - `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective`
     - `inference.networking.k8s.io/v1, Kind=InferencePool`

## Identity Model

1. Pool identity (`PoolOnly`). One SPIFFE identity representing the serving pool pods.
2. Objective identity (`PerObjective`). One SPIFFE identity per [`InferenceObjective`][gaie-inferenceobjective], representing the model endpoint at the GAIE layer even when multiple objectives share the same pool.

`PoolOnly` and `PerObjective` are the only identity boundaries in `kleym`.

### Container Level Enforcement

One model per container makes model identity enforceable. SPIRE Kubernetes workload attestation supports container scoped selectors such as container name and container image, so `kleym` can bind an objective identity to a specific container inside a pod rather than the pod as a whole. This is the mechanism that gives `PerObjective` mode meaningful discrimination when multiple objectives share a pool.

## Constraint

Multiple [`ClusterSPIFFEID`][clusterspiffeid] resources can select the same pod set, which can result in multiple identities applying to the same pods. Some workloads only support one SVID reliably. Clusters that require per objective identities may need to restrict or disable any default identity that would collide.
Some downstream consumers and sidecars behave as single identity consumers. Multiple matching [`ClusterSPIFFEID`][clusterspiffeid] objects may be valid from SPIRE's perspective but still operationally unsafe for a given serving stack. `kleym` only prevents deterministic collision cases it can prove. Cluster operators remain responsible for disabling overlapping default identities outside `kleym`.

## MVP API Surface

External CRDs consumed

Compatibility matrix for GAIE inputs (by GVK):

1. `inference.networking.k8s.io/v1, Kind=InferencePool` (preferred).
2. `inference.networking.x-k8s.io/v1alpha2, Kind=InferencePool` (compatible).
3. `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective` (currently required in most released GAIE versions).
4. `inference.networking.k8s.io/v1, Kind=InferenceObjective` (compatible when present).

`kleym` CRD

`InferenceIdentityBinding` expresses identity intent for a single [`InferenceObjective`][gaie-inferenceobjective].

`InferenceIdentityBinding` spec

1. `targetRef` references an [`InferenceObjective`][gaie-inferenceobjective] in the same namespace.
2. `spiffeIDTemplate` optionally overrides the computed template. Default is deterministic and includes trust domain, namespace, and objective name.
3. `selectorSource` is `"DerivedFromPool"`. `kleym` derives pod selection from the objective `poolRef` and validates it.
4. `workloadSelectorTemplates` are required safety constraints. Every rendered [`ClusterSPIFFEID`][clusterspiffeid] must include at minimum the k8s namespace selector (`k8s:ns:<namespace>`) and k8s service account selector (`k8s:sa:<service-account>`). These safety selectors are always present, then intersected with the derived pool selection and, in `PerObjective` mode, the container discriminator.
5. `mode` is `"PoolOnly"` or `"PerObjective"`. Default is `"PerObjective"`.
6. `containerDiscriminator` (required when `mode` is `"PerObjective"`).
   - `type` is `"ContainerName"` (preferred) or `"ContainerImage"` (fallback). `ContainerName` maps to a SPIRE k8s workload selector `k8s:container-name:<value>`. `ContainerImage` maps to `k8s:container-image:<value>` and is weaker because a single image may serve multiple models.
   - `value` is the container name or image reference to match.
   - The container discriminator narrows the selected workload set so that each objective identity targets exactly one container within the pod.

`InferenceIdentityBinding` status

1. `computedSpiffeIDs` lists identities currently reconciled for the binding. Current behavior writes one entry: pool identity in `PoolOnly` mode or objective identity in `PerObjective` mode.
2. `renderedSelectors` shows the final selectors applied to [`ClusterSPIFFEID`][clusterspiffeid].
3. `conditions` include `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, `RenderFailure`. The `Conflict` condition uses reason `IdentityCollision` when two objectives in `PerObjective` mode resolve to the same pod set and the same container name.

## Controller Behavior

1. Discover supported GAIE objective and pool GVKs served by the cluster (`GVK = GroupVersionKind`); watch only discovered GVKs.
   - Startup fails only when none of the supported GAIE objective/pool GVKs are available.
2. Watch `InferenceIdentityBinding`, plus discovered [`InferenceObjective`][gaie-inferenceobjective] and discovered [`InferencePool`][gaie-inferencepool] GVKs.
   - Example: if the cluster serves only `InferenceObjective` in `inference.networking.x-k8s.io/v1alpha2`, `kleym` watches that GVK and skips `inference.networking.k8s.io/v1` objective.
3. Resolve `targetRef` to [`InferenceObjective`][gaie-inferenceobjective], then resolve `poolRef` to [`InferencePool`][gaie-inferencepool].
4. Derive pod selection from [`InferencePool`][gaie-inferencepool], then intersect with the mandatory safety selectors (namespace and service account) and, in `PerObjective` mode, the container discriminator.
5. Detect identity collisions: if two `InferenceIdentityBinding` resources in `PerObjective` mode would match the same pod set and the same `container-name` value, set the `Conflict` condition with reason `IdentityCollision` on both resources and refuse to reconcile either until the collision is resolved.
6. Reconcile one or more [`ClusterSPIFFEID`][clusterspiffeid] resources in `spire.spiffe.io` using the computed SPIFFE IDs and validated selectors.
7. Update status and emit events for conflicts, unsafe selection, identity collisions, and render failures.
8. Treat infrastructure-not-ready states such as missing required CRDs as transient by retrying reconciliation on a timer so recovery does not depend on unrelated watch events.
9. On `InferenceIdentityBinding` deletion, remove managed [`ClusterSPIFFEID`][clusterspiffeid] children first and keep the binding finalizer until a follow-up list confirms no managed children remain.

## Multi Tenant Safety

1. `InferenceIdentityBinding` is namespaced and only references objects in the same namespace.
2. Derived selectors must be proven to stay within the namespace. If they can match outside, reconciliation is refused and `UnsafeSelector` is set.
3. Ambiguous bindings where the derived selection cannot be proven to correspond to the referenced pool are refused.

## Multiple Objectives to One Pod Set

GAIE commonly maps multiple objectives to one pool. In `kleym`, the pool defines where it runs and the objective defines what it is. `kleym` can produce distinct objective identities while selectors still target the same pods.

When two objectives share a pool, the container discriminator is what keeps their identities distinct. Each objective must point to a different container name within the pod. If two objectives resolve to the same pod set and the same `container-name`, reconciliation is refused on both with reason `IdentityCollision` until the conflict is corrected, for example by assigning each model to its own container.

## Acceptance Criteria

1. In a cluster with existing GAIE [`InferencePool`][gaie-inferencepool] and [`InferenceObjective`][gaie-inferenceobjective] resources, creating an `InferenceIdentityBinding` creates stable [`ClusterSPIFFEID`][clusterspiffeid] resources and remains stable under resync.
2. Multiple objectives referencing one pool produce distinct SPIFFE IDs without unsafe selector expansion.
3. Overly broad or malicious selector expansion is rejected with clear status conditions.
4. `kleym` does not create or modify inference deployments, pools, routes, [`Gateway`][gateway-api-gateway], [`HTTPRoute`][gateway-api-httproute], or schedulers.

## Licensing

1. License is Apache 2.0.

## References

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[spiffe-overview]: https://spiffe.io/docs/latest/spiffe-about/overview/
[spire-concepts]: https://spiffe.io/docs/latest/spire-about/spire-concepts/
[spire-controller-manager]: https://github.com/spiffe/spire-controller-manager
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[gaie-inferenceobjective]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/
[gaie-inferencemodel-legacy]: https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/
[gateway-api-gateway]: https://gateway-api.sigs.k8s.io/api-types/gateway/
[gateway-api-httproute]: https://gateway-api.sigs.k8s.io/api-types/httproute/
