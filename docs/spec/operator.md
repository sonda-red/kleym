---
title: Operator Spec
weight: 10
description: "Authoritative operator contract for InferenceIdentityBinding, GAIE pool resolution, SPIFFE ID rendering, selector safety, managed output, and status."
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

`InferenceIdentityBinding` is namespaced. Pool references stay in that namespace.

1. `poolRef` references one [`InferencePool`][gaie-inferencepool]. The pool is the required workload anchor and selector provenance source.
2. `serviceAccountName` is required. It scopes the rendered identity path, and Kleym renders safety selectors internally as `k8s:ns:<binding namespace>` and `k8s:sa:<serviceAccountName>`.
3. SPIFFE IDs are always deterministic under the configured trust domain: `spiffe://<trustDomain>/ns/<namespace>/sa/<serviceAccountName>/inference/<anchor-kind>/<anchor-name>`.
4. Status records `trustDomain`, `clusterSPIFFEIDClassName`, `computedSpiffeIDs`, `renderedSelectors`, `renderedClusterSPIFFEID`, and conditions. The current pool-only condition taxonomy is limited to `Ready`, `InvalidRef`, `UnsafeSelector`, and `RenderFailure`.
5. `trustDomain` and `clusterSPIFFEIDClassName` record the operator config values used for the latest status update. They are observation data for read-only inspection compatibility; they do not make trust domain or class name per-binding spec intent.
6. The CRD exposes printer columns for `POOL`, `READY`, `REASON`, and `SPIFFE ID` so `kubectl get inferenceidentitybindings.kleym.sonda.red -A` is the primary binding list view.

Field details live in [API Reference][api-reference]. Condition details live in [Conditions Reference][conditions-reference].

## Resolved Inference Target Contract

Identity and selector rendering consume a resolved inference target after the
referenced source object has been read. A GAIE `InferencePool` resolves to
identity anchor kind `pool`, with the anchor name equal to the pool name.

The canonical identity path contains the binding namespace, required service
account, and resolved identity anchor. Source provenance such as the raw source
group, version, or kind and the `InferenceIdentityBinding` name remains outside
the SPIFFE ID. Two bindings for the same namespace and pool but different
service accounts therefore render different SPIFFE IDs.

## Rendered Selector Contract

The current selector contract is pool-only. A binding accepts only
`spec.poolRef` and `spec.serviceAccountName`; it does not accept user-supplied
selector lists or selector source modes.

Every rendered identity selector set is assembled from these sources:

1. Mandatory namespace selector rendered internally from the binding namespace:
   `k8s:ns:<binding namespace>`.
2. Mandatory service-account selector rendered internally from
   `spec.serviceAccountName`: `k8s:sa:<serviceAccountName>`.
3. Pool-derived selectors rendered from the referenced `InferencePool`
   `spec.selector.matchLabels`.

The only pool selector compatibility form is an existing flat string map under
`spec.selector`; Kleym normalizes it to `matchLabels` before rendering. Pool
labels render directly to `k8s:pod-label:<key>:<value>` workload selectors.

Unsupported pool selector input is refused, including:

- missing or empty `spec.selector`
- empty `matchLabels`
- empty or malformed label keys
- empty, malformed, or non-string label values
- label values with leading or trailing whitespace
- any `matchExpressions` field, including an empty array
- selector shapes that cannot be decoded into deterministic string
  `matchLabels`

Rendered selector sets are canonical. Kleym removes duplicate selector strings
and sorts the remaining strings lexicographically before writing
`status.renderedSelectors`, managed `ClusterSPIFFEID`
`spec.workloadSelectorTemplates`, or the rendered selector fingerprint in
`status.renderedClusterSPIFFEID.selectorFingerprint`.
The canonical set preserves provenance by selector form: namespace and
service-account selectors are binding-derived, and `k8s:pod-label` selectors are
pool-derived.

Malformed or unsupported pool selector input fails reconciliation with
`UnsafeSelector=True` reason `InvalidPoolSelector`. A rendered selector set that
would omit or escape the mandatory namespace or service-account boundary fails
with `UnsafeSelector=True` reason `UnsafeSelector`. Selector failures produce no
managed output and clear `computedSpiffeIDs`, `renderedSelectors`, and
`renderedClusterSPIFFEID` in status.

## Rendered Managed Status

On successful reconciliation, `status.renderedClusterSPIFFEID` exposes the core
managed `ClusterSPIFFEID` details that status-only clients need for inspection:
the deterministic managed resource name, the rendered SPIFFE ID, a
`sha256:<hex>` fingerprint of the canonical selector set, and the observed
managed resource generation when it is available from Kubernetes.

`status.renderedClusterSPIFFEID.spiffeID` is populated from the same rendered
identity used for `status.computedSpiffeIDs`; it is not a second SPIFFE state.
`observedGeneration` is omitted when Kubernetes has not reported a persisted
generation for the managed resource. Kleym does not write `0` as a fake
generation. If the managed resource cannot be listed, created, updated, or
deleted because the API request itself fails, reconciliation reports
`RenderFailure=True` with reason `ManagedOutputApplyFailed`, clears
`computedSpiffeIDs`, `renderedSelectors`, and `renderedClusterSPIFFEID`, and
returns the API error so reconciliation retries.

## Required Behavior

1. Discover the supported GAIE pool GVK served by the cluster and watch it.
2. Fail startup when the supported `InferencePool` GVK is not available.
3. Resolve `poolRef` only to documented supported GAIE groups.
4. Resolve the referenced pool into the inference target identity anchor and pod-selector inputs.
5. Combine resolved target selectors with internal namespace and service-account safety selectors.
6. Refuse unsafe selectors. If the selector set cannot be proven to stay within the binding namespace and required service account boundary, set `UnsafeSelector` and produce no managed output.
7. Render the SPIFFE ID and managed `ClusterSPIFFEID` shape deterministically. Rendered output fields are documented in [Managed Resources][managed-resources].
8. After startup succeeds, treat missing managed-output CRDs and infrastructure-not-ready states as transient by retrying reconciliation on a timer.
9. On deletion, delete managed `ClusterSPIFFEID` children first and keep the binding finalizer until a follow-up list confirms no managed children remain.

Selector rationale is expanded in [Selector Safety][selector-safety].

## Condition Taxonomy

`InferenceIdentityBinding.status.conditions` is a stable machine-readable contract for reference resolution, selector safety, identity rendering, managed-output application, and reconciliation readiness.

Allowed condition types and reasons:

| Condition | Status | Allowed reasons |
| --- | --- | --- |
| `Ready` | `True` | `Reconciled` |
| `Ready` | `False` | The same primary failure reason used by the active failure condition. |
| `Ready` | `Unknown` | `Initializing` |
| `InvalidRef` | `True` | `InvalidPoolRef`, `TargetPoolNotFound`, `InferencePoolCRDMissing` |
| `UnsafeSelector` | `True` | `InvalidPoolSelector`, `UnsafeSelector` |
| `RenderFailure` | `True` | `MissingTrustDomain`, `InvalidServiceAccountName`, `InvalidSPIFFEID`, `ClusterSPIFFEIDCRDMissing`, `ManagedOutputApplyFailed` |
| `InvalidRef`, `UnsafeSelector`, `RenderFailure` | `False` | `Resolved` |
| `InvalidRef`, `UnsafeSelector`, `RenderFailure` | `Unknown` | `Initializing` |

On success, `Ready=True` uses `Reconciled`, every failure condition is `False` with `Resolved`, and rendered identity plus managed-resource status is populated.

On failure, `Ready=False` uses the same reason and message as the one active failure condition. Exactly one of `InvalidRef`, `UnsafeSelector`, or `RenderFailure` is `True`, and `computedSpiffeIDs`, `renderedSelectors`, plus `renderedClusterSPIFFEID` are cleared.

Dependency-unavailable states use the same taxonomy:

- Missing GAIE `InferencePool` CRD during pool discovery or pool resolution is `InvalidRef=True` with reason `InferencePoolCRDMissing`. Startup discovery fails when no supported GAIE pool GVK is served.
- Missing SPIRE Controller Manager `ClusterSPIFFEID` CRD during reconcile is `RenderFailure=True` with reason `ClusterSPIFFEIDCRDMissing` and the reconcile retries on a timer.
- Generic managed `ClusterSPIFFEID` list, create, update, or delete API failures are `RenderFailure=True` with reason `ManagedOutputApplyFailed`. The reconcile returns the API error for retry and clears rendered identity plus managed-resource status from the failed attempt.

## Safety Invariants

1. `InferenceIdentityBinding` is namespaced.
2. Pool references stay in the binding namespace.
3. Unsafe selectors are refused.
4. Deletion keeps the finalizer until managed children are gone.
5. `kleym-operator` does not create or modify inference deployments, pools, routes, gateways, schedulers, or policy resources.

## References

[clusterspiffeid]: https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md
[gaie-inferencepool]: https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/
[api-reference]: /reference/api/
[conditions-reference]: /reference/conditions/
[dependencies]: /reference/dependencies/
[gaie-compatibility]: /reference/gaie-compatibility/
[managed-resources]: /reference/resources/
[selector-safety]: /design/selector-safety/
