---
title: Operator Spec
weight: 10
description: "Authoritative operator contract for InferenceIdentityBinding, identity boundaries, GAIE pool resolution, SPIFFE ID rendering, selector safety, managed output, and status."
---

`kleym-operator` is an inference identity boundary compiler. It translates declared inference intent into deterministic Secure Production Identity Framework for Everyone (SPIFFE) identities and writes SPIFFE Runtime Environment (SPIRE) Controller Manager [`ClusterSPIFFEID`][clusterspiffeid] resources.

Kleym stops at identity registration. It does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or write SPIRE registration entries directly.

## Scope

Kleym owns `InferenceIdentityBinding`, GAIE input resolution, identity-boundary resolution, selector safety, managed `ClusterSPIFFEID` reconciliation, status, events, and finalizer cleanup.

Kleym does not own inference workloads, schedulers, routes, gateways, serving behavior, Envoy, OPA, OAuth, OIDC, SPIRE Server, SPIRE Agent, credential issuance, authorization, or audit decisions.

Dependency facts live in [Dependencies][dependencies]. Supported GAIE inputs live in [GAIE Compatibility][gaie-compatibility].

## Binding Write Authorization Boundary

`InferenceIdentityBinding` is a namespaced resource, but `kleym-operator` reconciles each successful binding into a cluster-scoped SPIRE Controller Manager `ClusterSPIFFEID` resource. Create, update, or patch permission on `InferenceIdentityBinding` is therefore a privileged namespace capability: it delegates permission to request Kleym-managed SPIRE identity registration for workloads in that namespace.

Kubernetes RBAC and admission policy decide who may create, update, or patch bindings. Kleym does not perform tenant authorization for binding authors. Broad application-developer write access to `InferenceIdentityBinding` should be a deliberate delegation of identity-registration authority.

Selector safety and boundary exclusivity are separate from authorization. Namespace, service-account, and pool selectors constrain the rendered `ClusterSPIFFEID` workload match, but they are not the authorization decision for whether a user may request identity registration.

## Operator Configuration

`kleym-operator` requires install-level identity configuration at startup. Command-line flags are the canonical configuration surface. Environment variables are startup-only fallbacks when the matching flag is omitted; they are not watched or reloaded after process start.

| Flag | Environment fallback | Required | Behavior |
| --- | --- | --- | --- |
| `--trust-domain=<value>` | `KLEYM_TRUST_DOMAIN` | yes | Sets the SPIRE Server trust domain used when rendering every SPIFFE ID. The value must not include `spiffe://`, must not contain `/`, and must not include leading or trailing whitespace. |
| `--clusterspiffeid-class-name=<value>` | `KLEYM_CLUSTERSPIFFEID_CLASS_NAME` | no | Sets `spec.className` on every managed `ClusterSPIFFEID`. When empty, Kleym omits `spec.className` and keeps classless output. |

Explicit flags take precedence over environment variables, including explicit empty values. Missing trust domain from both `--trust-domain` and `KLEYM_TRUST_DOMAIN` fails startup with `trustDomain must be configured before Kleym can render SPIFFE IDs`. Values loaded from environment variables use the same validation rules as flag values.

`trustDomain` and `ClusterSPIFFEID` class are deployment concerns, not per-binding inference identity intent. They are not fields in `InferenceIdentityBinding.spec`.

When `--clusterspiffeid-class-name` is empty, SPIRE Controller Manager must be configured to watch classless `ClusterSPIFFEID` resources, for example with its `watchClassless` behavior. When a class name is set, SPIRE Controller Manager must watch that class.

## API Contract

`InferenceIdentityBinding` is namespaced. Pool references stay in that namespace.

1. `poolRef` references one [`InferencePool`][gaie-inferencepool]. The pool is the required workload anchor and selector provenance source.
2. `serviceAccountName` is required and admission-validated as a DNS-1123 subdomain with a maximum length of 253 characters. It scopes both the SPIFFE ID path and the mandatory `k8s:sa:<serviceAccountName>` selector.
3. `identityBoundary.variant` is required and must be a valid, nonempty Kubernetes label value. Kleym owns the fixed boundary label key `identity.kleym.sonda.red/variant`; callers cannot select a different key.
4. SPIFFE IDs are deterministic under the configured trust domain: `spiffe://<trustDomain>/ns/<namespace>/sa/<serviceAccountName>/inference/pool/<pool-name>/variant/<variant>`.
5. Status records operator configuration, rendered output, conflicts, and conditions. The status rules are defined in [Status Contract](#status-contract).
6. The CRD exposes printer columns for `POOL`, `BOUNDARY`, `READY`, `REASON`, and `SPIFFE ID` so `kubectl get inferenceidentitybindings.kleym.sonda.red -A` is the primary binding list view.

[API Reference][api-reference] and [Conditions Reference][conditions-reference] document the implemented API and condition surfaces.

## Resolved Inference Target Contract

Identity and selector rendering consume a resolved inference target after the referenced source object has been read. A GAIE `InferencePool` resolves to identity anchor kind `pool`, with the anchor name equal to the pool name.

The canonical SPIFFE ID contains the binding namespace, required service account, and resolved identity anchor. Source provenance such as the raw source group, version, or kind and the `InferenceIdentityBinding` name remains outside the SPIFFE ID. Two bindings for the same namespace and pool but different service accounts therefore render different SPIFFE IDs.

An `InferencePool` is the broad model-serving group. Each `InferenceIdentityBinding` claims one label-defined workload variant within that pool, such as `prefill` or `decode`. A variant is a workload subset, not a separate resource or caller authorization model.

## Rendered Selector Contract

A binding accepts `spec.poolRef`, `spec.serviceAccountName`, and `spec.identityBoundary`; it does not accept user-supplied selector lists or selector source modes.

Every rendered identity selector set is assembled from these sources:

1. Mandatory namespace selector rendered internally from the binding namespace: `k8s:ns:<binding namespace>`.
2. Mandatory service-account selector rendered internally from `spec.serviceAccountName`: `k8s:sa:<serviceAccountName>`.
3. Pool-derived selectors rendered from the referenced `InferencePool` `spec.selector.matchLabels`.
4. Mandatory boundary selector rendered from `spec.identityBoundary.variant`: `k8s:pod-label:identity.kleym.sonda.red/variant:<variant>`.

The only pool selector compatibility form is an existing flat string map under `spec.selector`; Kleym normalizes it to `matchLabels` before rendering. Pool labels render directly to `k8s:pod-label:<key>:<value>` workload selectors.

Unsupported pool selector input is refused, including:

- missing or empty `spec.selector`
- empty `matchLabels`
- empty or malformed label keys
- empty, malformed, or non-string label values
- label values with leading or trailing whitespace
- any `matchExpressions` field, including an empty array
- selector shapes that cannot be decoded into deterministic string `matchLabels`

Rendered selector sets are canonical. Kleym removes duplicate selector strings and sorts the remaining strings lexicographically before writing `status.renderedSelectors`, managed `ClusterSPIFFEID` `spec.workloadSelectorTemplates`, or the rendered selector fingerprint in `status.renderedClusterSPIFFEID.selectorFingerprint`.

The complete normalized pool selector remains the workload match. Kleym adds the fixed-key identity-boundary selector exactly once to select one variant within that pool. The boundary label proves exclusivity; it does not replace other pool labels in `ClusterSPIFFEID.spec.podSelector` or `spec.workloadSelectorTemplates`.

Malformed or unsupported pool selector input fails reconciliation with `UnsafeSelector=True` reason `InvalidPoolSelector`. A selector set that would omit or escape the mandatory namespace or service-account boundary fails with `UnsafeSelector=True` reason `UnsafeSelector`. Selector failures produce no managed output and clear rendered output status.

## Identity Boundary and Exclusivity

The binding declares an identity boundary. The resolved boundary is:

```text
namespace
service account
variant
```

The declared variant is rendered under the operator-owned `identity.kleym.sonda.red/variant` key as a mandatory Pod-label selector. A missing, empty, malformed, or whitespace-padded variant is refused with `UnsafeSelector=True` reason `InvalidIdentityBoundary`.

For any two bindings with different rendered SPIFFE IDs, exclusivity is proven only when at least one condition is true:

```text
namespace differs
OR
service account differs
OR
variant differs
```

Every other relationship is a conflict. Different values of the fixed Kubernetes label key are structurally disjoint because one workload cannot hold two values for that key. Kleym does not use general selector intersection as a fallback.

Two bindings that render the same SPIFFE ID are duplicate identity claims. They are conflicts even when their selectors or boundary declarations are equal.

## Conflict Behavior

Kleym evaluates potentially conflicting bindings from declared binding and pool state. It forms a conflict group only from bindings that fail the pairwise exclusivity invariant; bindings already proven exclusive are not members. A deleting binding remains a competitor until its managed output is confirmed absent; peers must not recreate output while that deletion is pending.

A conflict member has:

```text
Ready=False
Conflict=True
managed ClusterSPIFFEID absent
rendered output status cleared
conflict references populated
```

Kleym removes every managed `ClusterSPIFFEID` owned by conflict members and confirms absence before settling the conflict state. If output deletion fails, reconciliation reports the API error and retries; it must not report a settled conflict state that implies output absence.

Output is recreated only after the complete conflict group becomes exclusive or a deleting member's output absence is confirmed. Binding creation, update, deletion, and referenced pool selector changes must converge every affected peer to the appropriate state.

Kleym removes registration intent. It does not claim immediate invalidation of an SVID already issued from a prior registration.

## Status Contract

`status.conflicts` is present only when `Conflict=True`. Each item describes one peer and one precise cause:

```yaml
bindingName: <peer name>
cause: VariantReuse | DuplicateSPIFFEID
spiffeID: <peer rendered SPIFFE ID>
variant: <peer variant>
```

All four fields are required. Because conflicts are evaluated only against same-namespace peers, `bindingName` is sufficient to identify the peer. Items are sorted by peer binding name, cause, SPIFFE ID, and variant. A binding may have multiple items for multiple peers or causes.

`Conflict=True` uses `DuplicateSPIFFEID` when any item has that cause; otherwise it uses `VariantConflict`. The condition reason is the coarse conflict class, while each item records the precise cause.

`status.computedSpiffeIDs`, `status.renderedSelectors`, and `status.renderedClusterSPIFFEID` are populated only when `Ready=True`. They are cleared for selector failures, conflicts, and managed-output API failures.

`status.pendingClusterSPIFFEID` records a deterministic output `name` and controller-generated unique `claimID` after a successful NotFound read and before Kleym calls `Create`. Kleym writes the same claim ID to the new object's `kleym.sonda.red/ownership-claim-id` annotation. Rendered output remains cleared while the claim is pending. If `Create` succeeds, times out ambiguously, or is followed by a failed status patch or process stop, retry promotes the pending record only when the live same-name object carries the matching claim and Kubernetes has assigned it a UID. A live same-name object with a missing or different claim is foreign.

`status.ownedClusterSPIFFEID` is the confirmed ownership record. It contains the deterministic `name` and Kubernetes `uid` of the exact managed object incarnation. Both values must match the live object before Kleym may update or delete it. A live same-name object with a different UID is foreign and is never updated, deleted, or adopted; the mismatch proves only that the previously recorded incarnation is absent. The claim ID is an incarnation-correlation token for pending-create recovery, not authentication against cluster administrators.

Allowed condition types and reasons:

| Condition | Status | Allowed reasons |
| --- | --- | --- |
| `Ready` | `True` | `Reconciled` |
| `Ready` | `False` | The same primary failure reason used by the active failure condition. |
| `Ready` | `Unknown` | `Initializing` |
| `InvalidRef` | `True` | `InvalidPoolRef`, `TargetPoolNotFound`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | `True` | `InvalidPoolSelector`, `UnsafeSelector`, `InvalidIdentityBoundary` |
| `Conflict` | `True` | `VariantConflict`, `DuplicateSPIFFEID` |
| `RenderFailure` | `True` | `MissingTrustDomain`, `InvalidServiceAccountName`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing`, `ManagedOutputApplyFailed` |
| `InvalidRef` | `False` | `Resolved` |
| `UnsafeSelector` | `False` | `Resolved` |
| `Conflict` | `False` | `Resolved` |
| `RenderFailure` | `False` | `Resolved` |
| `InvalidRef`, `UnsafeSelector`, `Conflict`, `RenderFailure` | `Unknown` | `Initializing` |

Exactly one primary failure condition is `True`. `Ready=False` uses the same reason and message. Conflict causes are `VariantReuse` and `DuplicateSPIFFEID`.

## Rendered Managed Status

On successful reconciliation, `status.renderedClusterSPIFFEID` exposes the deterministic managed `ClusterSPIFFEID` name, rendered SPIFFE ID, a `sha256:<hex>` fingerprint of the canonical selector set, and the observed managed-resource generation when Kubernetes reports one.

The persisted pending and confirmed records form the durable managed-output ownership protocol. Neither rendered output status, deterministic rendering, desired-spec equality, generation, binding references, nor managed-by and binding labels prove ownership. A pre-existing deterministic-name object absent from both records is foreign and must not be updated or deleted, even when every descriptive field matches the binding.

Ownership classification is fail closed:

- No record plus an absent desired name permits Kleym to persist a fresh pending claim before `Create`; a live object is foreign.
- A pending record plus an absent live object retries `Create` with the same claim. A matching live claim is promoted to confirmed name-plus-UID ownership without a second `Create`; a missing or different live claim is foreign.
- A confirmed record plus a matching live name and UID permits in-sync observation, drift correction, or cleanup. NotFound proves the recorded incarnation absent. A different live UID also proves the recorded incarnation absent but leaves the same-name foreign replacement untouched.
- A terminating object is still present until a later NotFound read. Delete uses a UID precondition and ownership remains recorded until absence is confirmed.

Transient read, create, update, and delete errors, including API discovery `NoMatch`, are uncertainty: they preserve the pending or confirmed record and binding finalizer. Only a successful live observation or NotFound response changes ownership state.

When an identity change produces a different deterministic name, Kleym deletes only a claim-matched pending object or name-and-UID-matched confirmed object and confirms that exact recorded incarnation is absent before reserving or creating the replacement. A same-name UID-mismatched foreign object is left untouched and does not prevent creation under the new desired name. While deletion of a matching prior object is pending, the binding retains the confirmed record, clears rendered output status, reports `Ready=Unknown` with reason `Initializing`, and requeues.

`status.renderedClusterSPIFFEID.spiffeID` is populated from the same rendered identity used for `status.computedSpiffeIDs`; it is not a second SPIFFE state. `observedGeneration` is omitted when Kubernetes has not reported a persisted generation.

If a managed resource cannot be read, created, updated, or deleted because the API request fails, or a same-name object is classified as foreign, reconciliation reports `RenderFailure=True` with reason `ManagedOutputApplyFailed` and a precise operation or ownership-mismatch message, clears rendered output status, and returns the error for retry. API uncertainty preserves pending and confirmed ownership. `meta.IsNoMatchError` is retryable API uncertainty, not confirmation that output is absent; it never clears ownership or permits finalizer removal.

## Required Behavior

1. Discover the supported GAIE pool GVK served by the cluster and watch it.
2. Fail startup when the supported `InferencePool` GVK is not available.
3. Resolve `poolRef` only to documented supported GAIE groups.
4. Normalize the referenced pool selector, validate the declared variant, and preserve the complete selector plus mandatory fixed-key variant selector for rendering.
5. Evaluate boundary exclusivity before managed output creation or update.
6. Refuse selector failures and conflicts. Both states produce no managed output.
7. Render the SPIFFE ID and managed `ClusterSPIFFEID` shape deterministically when the binding is exclusive.
8. After startup succeeds, treat missing managed-output CRDs and infrastructure-not-ready states as transient by retrying reconciliation on a timer.
9. On deletion, delete only the exact claim- or UID-matched managed `ClusterSPIFFEID` incarnation and keep the binding finalizer until a follow-up read proves that recorded incarnation absent. A UID-mismatched foreign replacement survives finalization.

## Boundary Label Ownership

An identity boundary label is security-sensitive metadata. Permission to assign the boundary label or selected service account is therefore identity registration authority.

`identity.kleym.sonda.red/variant` is the operator-owned boundary label key. Cluster admission policy must restrict assignment and mutation of this label to platform-controlled actors. The label is immutable for the lifetime of a Pod; variant changes use replacement Pods.

Deployments must enforce this ownership with Kubernetes admission control. See
[Boundary Label Ownership][boundary-label-ownership] for an opt-in native
Kubernetes reference policy.

Kubernetes RBAC and admission control remain responsible for
`InferenceIdentityBinding` writes and service-account assignment. Kleym does
not mutate workloads, pools, or Pods and has no workload-mutation RBAC.

## CLI Boundary

The CLI remains read only and consumes operator status. It does not reconcile
resources or implement a separate identity-boundary exclusivity model. The
current inspection contract is defined in the [CLI Spec](/spec/cli/).

## Safety Invariants

1. `InferenceIdentityBinding` is namespaced.
2. Pool references stay in the binding namespace.
3. Namespace and service-account selectors are mandatory.
4. Full normalized pool selectors are preserved in managed output.
5. Different managed SPIFFE IDs must satisfy the identity-boundary exclusivity invariant.
6. Conflicting and duplicate identity claims retain no managed output.
7. Mutation and cleanup require durable pending-claim or confirmed name-plus-UID authorization for the exact live object incarnation.
8. Deletion keeps the finalizer through API uncertainty and until the recorded managed incarnation is proven absent.
9. `kleym-operator` does not create or modify inference deployments, pools, routes, gateways, schedulers, or policy resources.

## References

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[api-reference]: /reference/api/
[conditions-reference]: /reference/conditions/
[dependencies]: /reference/dependencies/
[gaie-compatibility]: /reference/gaie-compatibility/
[boundary-label-ownership]: /reference/boundary-label-ownership/
