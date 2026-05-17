---
title: Spec
weight: 80
---

`kleym-operator` makes inference workloads legible to workload identity by translating inference intent into deterministic Secure Production Identity Framework for Everyone (SPIFFE) identities. It compiles that intent into SPIFFE Runtime Environment (SPIRE) Controller Manager resources, primarily [`ClusterSPIFFEID`][clusterspiffeid].

Scope boundary: Kleym is an identity registration compiler project. The current implementation is `kleym-operator`, which stops at identity registration and selector provenance. Inference deployment, traffic routing, and policy evaluation stay in the inference stack, gateway, mesh, or external policy engines. `kleym-operator` does not configure Envoy, Envoy Gateway, kgateway, ext authz, ext proc, OPA, Cedar, OAuth, or OIDC.

Reference docs: [SPIFFE overview][spiffe-overview], [SPIRE concepts][spire-concepts], and [SPIRE Controller Manager][spire-controller-manager].

## Core Problem

Inference stacks can be deployed reliably, but identity registration remains manual, inconsistent, and hard to standardize across teams. Gateway API Inference Extension (GAIE) defines inference-specific objects and clearer responsibility boundaries, but it does not define identity semantics. `kleym-operator` bridges this gap by deriving stable SPIFFE ID templates and tenant safe selectors from GAIE resources.

## Core Value

1. Deterministic identities derived from GAIE metadata rather than ad hoc labels.
2. A single namespaced control surface for identity intent that works across heterogeneous inference stacks that share GAIE semantics.
3. Low operational risk by delegating issuance and rotation to SPIRE Controller Manager.

## Dependencies

1. SPIRE Server and SPIRE Agent.
2. SPIRE Controller Manager and its [`ClusterSPIFFEID`][clusterspiffeid] CRD.
3. `kleym-operator` writes [`ClusterSPIFFEID`][clusterspiffeid] and does not write SPIRE entries directly.

## Supported Downstream Pattern

1. SPIRE issues X.509 SVIDs and/or JWT SVIDs.
2. Envoy consumes identity through SDS or through components adjacent to the SPIRE Workload API.
3. External auth policy maps SPIFFE ID to route or model authorization.
4. Audit logging happens at the gateway or policy layer, not in `kleym-operator`.

## Preferred Inference Signal

GAIE objects are the primary signal.

1. [`InferencePool`][gaie-inferencepool] is the required workload anchor and selector provenance source for identity registration.
2. [`InferenceObjective`][gaie-inferenceobjective] is an optional model level subject. When used, it must reference the same [`InferencePool`][gaie-inferencepool] via `poolRef`.
3. [`InferenceModel`][gaie-inferencemodel-legacy] is treated as legacy.
4. At startup, `kleym-operator` discovers which supported GAIE GVKs are served by the cluster and watches only that subset.
   - `GVK` means `GroupVersionKind` (`<api-group>/<version>, Kind=<kind>`), for example:
     - `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective`
     - `inference.networking.k8s.io/v1, Kind=InferencePool`

## Identity Model

1. Pool identity (`PoolOnly`). One SPIFFE identity representing the serving pool pods.
2. Objective identity (`PerObjective`). One SPIFFE identity per [`InferenceObjective`][gaie-inferenceobjective], representing the model endpoint at the GAIE layer even when multiple objectives share the same pool.

`PoolOnly` and `PerObjective` are the only identity boundaries in `kleym-operator`.

### Container Level Enforcement

One model per container makes model identity enforceable. SPIRE Kubernetes workload attestation supports container scoped selectors such as container name and container image, so `kleym-operator` can bind an objective identity to a specific container inside a pod rather than the pod as a whole. This is the mechanism that gives `PerObjective` mode meaningful discrimination when multiple objectives share a pool.

## Constraint

Multiple [`ClusterSPIFFEID`][clusterspiffeid] resources can select the same pod set, which can result in multiple identities applying to the same pods. Some workloads only support one SVID reliably. Clusters that require per objective identities may need to restrict or disable any default identity that would collide.
Some downstream consumers and sidecars behave as single identity consumers. Multiple matching [`ClusterSPIFFEID`][clusterspiffeid] objects may be valid from SPIRE's perspective but still operationally unsafe for a given serving stack. `kleym-operator` only prevents deterministic collision cases it can prove. Cluster operators remain responsible for disabling overlapping default identities outside Kleym.

## MVP API Surface

External CRDs consumed

Compatibility matrix for GAIE inputs (by GVK):

1. `inference.networking.k8s.io/v1, Kind=InferencePool` (preferred).
2. `inference.networking.x-k8s.io/v1alpha2, Kind=InferencePool` (compatible).
3. `inference.networking.x-k8s.io/v1alpha2, Kind=InferenceObjective` (compatible when an objective subject is used).
4. `inference.networking.k8s.io/v1, Kind=InferenceObjective` (compatible when present).

Kleym CRD

`InferenceIdentityBinding` expresses identity intent for a single [`InferencePool`][gaie-inferencepool] and, in `PerObjective` mode, an [`InferenceObjective`][gaie-inferenceobjective] subject.

`InferenceIdentityBinding` spec

1. `poolRef` references an [`InferencePool`][gaie-inferencepool] in the same namespace. `poolRef.group` may constrain resolution to one supported GAIE InferencePool group.
2. `objectiveRef` optionally references an [`InferenceObjective`][gaie-inferenceobjective] in the same namespace. It is required when `mode` is `PerObjective`. When present, the referenced objective's `spec.poolRef` must point at the same pool as `poolRef`. `PoolOnly` does not require an `InferenceObjective` or the `InferenceObjective` CRD.
3. `spiffeIDTemplate` optionally overrides the computed template. Defaults are deterministic and mode-scoped:
   - `PoolOnly`: `spiffe://kleym.sonda.red/ns/<namespace>/pool/<pool-name>`
   - `PerObjective`: `spiffe://kleym.sonda.red/ns/<namespace>/objective/<objective-name>`
4. `selectorSource` is `"DerivedFromPool"`. `kleym-operator` derives pod selection directly from `poolRef` and validates it.
5. `workloadSelectorTemplates` are required user-supplied safety constraints. The controller renders these templates, then requires every rendered [`ClusterSPIFFEID`][clusterspiffeid] selector set to include at minimum the k8s namespace selector (`k8s:ns:<namespace>`) and k8s service account selector (`k8s:sa:<service-account>`). These selectors are validated, then intersected with the derived pool selection and, in `PerObjective` mode, the container discriminator.
6. `mode` is `"PoolOnly"` or `"PerObjective"`. Default is `"PerObjective"`.
7. `containerDiscriminator` (required when `mode` is `"PerObjective"`).
   - `type` is `"ContainerName"` (preferred) or `"ContainerImage"` (fallback). `ContainerName` maps to a SPIRE k8s workload selector `k8s:container-name:<value>`. `ContainerImage` maps to `k8s:container-image:<value>` and is weaker because a single image may serve multiple models.
   - `value` is the container name or image reference to match.
   - The container discriminator narrows the selected workload set so that each objective identity targets exactly one container within the pod.

`InferenceIdentityBinding` status

1. `computedSpiffeIDs` lists identities currently reconciled for the binding. Current behavior writes one entry: pool identity in `PoolOnly` mode or objective identity in `PerObjective` mode.
2. `renderedSelectors` shows the final selectors applied to [`ClusterSPIFFEID`][clusterspiffeid].
3. `conditions` include `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, `RenderFailure`. The `Conflict` condition uses reason `IdentityCollision` when two objectives in `PerObjective` mode resolve to the same pod set and the same container name.

## Controller Behavior

1. Discover supported GAIE objective and pool GVKs served by the cluster (`GVK = GroupVersionKind`); watch only discovered GVKs.
   - Startup fails when no supported [`InferencePool`][gaie-inferencepool] GVK is available. Objective GVKs are optional for `PoolOnly`.
2. Watch `InferenceIdentityBinding`, discovered [`InferencePool`][gaie-inferencepool] GVKs, and discovered [`InferenceObjective`][gaie-inferenceobjective] GVKs when present.
   - Example: if the cluster serves only `InferencePool` in `inference.networking.k8s.io/v1`, `kleym-operator` can reconcile `PoolOnly` bindings without watching objectives.
3. Resolve `poolRef` to a supported [`InferencePool`][gaie-inferencepool] GVK. If `poolRef.group` is set, it must be one of the documented supported GAIE InferencePool groups.
4. Resolve `objectiveRef` when present or required by `PerObjective`. If `objectiveRef.group` is set, it must be one of the documented supported GAIE InferenceObjective groups. Validate that the objective's `spec.poolRef` points at the resolved `poolRef`.
5. Derive pod selection from [`InferencePool`][gaie-inferencepool], then intersect it with the validated user-supplied safety selectors (namespace and service account) and, in `PerObjective` mode, the container discriminator.
6. Detect identity collisions: if two `InferenceIdentityBinding` resources in `PerObjective` mode would match the same pod set and the same `container-name` value, set the `Conflict` condition with reason `IdentityCollision` on both resources and refuse to reconcile either until the collision is resolved.
7. Reconcile one or more [`ClusterSPIFFEID`][clusterspiffeid] resources in `spire.spiffe.io` using the computed SPIFFE IDs and validated selectors.
   - `spec.spiffeIDTemplate`: the fully rendered SPIFFE ID
   - `spec.podSelector`: the validated pod selector from the referenced pool
   - `spec.workloadSelectorTemplates`: rendered safety selectors, pool-derived selectors, and optional container discriminator
   - `spec.hint`: the originating binding reference (`<namespace>/<binding-name>`) for traceability
   - `spec.fallback`: always `false` (`kleym-operator` manages explicit per-objective identities, not fallback entries)
8. Update status and emit events for conflicts, unsafe selection, identity collisions, and render failures.
9. Treat infrastructure-not-ready states such as missing required CRDs as transient by retrying reconciliation on a timer so recovery does not depend on unrelated watch events.
10. On `InferenceIdentityBinding` deletion, remove managed [`ClusterSPIFFEID`][clusterspiffeid] children first and keep the binding finalizer until a follow-up list confirms no managed children remain.

## Multi Tenant Safety

1. `InferenceIdentityBinding` is namespaced and only references pools and objectives in the same namespace.
2. Derived selectors must be proven to stay within the namespace. If they can match outside, reconciliation is refused and `UnsafeSelector` is set.
3. Ambiguous bindings where the derived selection cannot be proven to correspond to the referenced pool are refused.

## Multiple Objectives to One Pod Set

GAIE commonly maps multiple objectives to one pool. In Kleym, the pool defines where it runs and the objective defines what it is. `kleym-operator` can produce distinct objective identities while selectors still target the same pods.

When two objectives share a pool, the container discriminator is what keeps their identities distinct. Each objective must point to a different container name within the pod. If two objectives resolve to the same pod set and the same `container-name`, reconciliation is refused on both with reason `IdentityCollision` until the conflict is corrected, for example by assigning each model to its own container.

## Acceptance Criteria

1. In a cluster with an existing GAIE [`InferencePool`][gaie-inferencepool], creating a `PoolOnly` `InferenceIdentityBinding` creates a stable [`ClusterSPIFFEID`][clusterspiffeid] resource and remains stable under resync.
2. Multiple objectives referencing one pool produce distinct SPIFFE IDs without unsafe selector expansion.
3. Overly broad or malicious selector expansion is rejected with clear status conditions.
4. `kleym-operator` does not create or modify inference deployments, pools, routes, [`Gateway`][gateway-api-gateway], [`HTTPRoute`][gateway-api-httproute], or schedulers.

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
