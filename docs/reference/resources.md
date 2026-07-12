---
title: Managed Resources
weight: 30
description: "Managed ClusterSPIFFEID output reference for labels, owner references, SPIFFE IDs, selectors, class names, and reconciliation ownership."
aliases:
  - /operator/reference/resources/
---

## Primary Managed Output

`kleym-operator` manages
[`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md)
resources in `spire.spiffe.io`.

## Rendered Field Mapping

| Field | Rendered value |
| --- | --- |
| `spec.spiffeIDTemplate` | Fully rendered SPIFFE ID. |
| `spec.podSelector` | Validated pool selector from the referenced pool, normalized to `matchLabels` when the compatibility flat string-map form is used. |
| `spec.workloadSelectorTemplates` | Canonical selector set: namespace, service account, complete pool-derived labels, and exactly one boundary-label selector, de-duplicated and sorted. |
| `spec.className` | Rendered only when `kleym-operator` is configured with `--clusterspiffeid-class-name`. When omitted, SPIRE Controller Manager must watch classless resources. |
| `spec.fallback` | `false` for all managed identities. |
| `spec.hint` | Originating binding reference in the form `<namespace>/<binding-name>`. |
| JWT-SVID-related fields | Not rendered today. Requires a user story and SPIRE Controller Manager/SPIRE version gate. |

Selector provenance and unsupported pool selector inputs are defined by the
[Operator Spec](/spec/operator/#rendered-selector-contract). Managed resources
do not add selector sources beyond that contract.

Managed `ClusterSPIFFEID` objects are labeled with:

- `kleym.sonda.red/managed-by=kleym`
- `kleym.sonda.red/binding-name=<binding-name>`
- `kleym.sonda.red/binding-namespace=<binding-namespace>`

Newly created objects also carry the provenance annotation
`kleym.sonda.red/ownership-claim-id=<claimID>`. The controller preserves this
annotation rather than rewriting it during drift correction.

The controller also uses the finalizer
`kleym.sonda.red/inferenceidentitybinding-finalizer` to clean up managed
`ClusterSPIFFEID` objects on deletion.

The persisted `status.pendingClusterSPIFFEID{name,claimID}` and
`status.ownedClusterSPIFFEID{name,uid}` records form the durable ownership
protocol for managed-output reads, drift correction, and cleanup. Pending
creation requires an annotation claim match before promotion. Confirmed update
or deletion requires both the name and live Kubernetes UID to match. Rendered
output status, deterministic spec equality, generation, and managed-by or
binding labels are descriptive and never prove ownership.

A deterministic-name object absent from both records is foreign. A live object
whose claim differs from a pending record or whose UID differs from confirmed
ownership is also foreign and is refused without update or deletion. The stale
record may be cleared because the recorded incarnation is proven absent, but
the replacement is not adopted. Transient API failures and `NoMatch` preserve
ownership and finalizer state. If an identity change produces a new
deterministic name, the controller confirms the exact recorded old incarnation
is absent before it reserves, creates, and confirms the replacement.

## Naming

Managed `ClusterSPIFFEID` names are deterministic and derived from:

- the `kleym-operator` controller name
- binding namespace
- binding name
- rendered identity anchor kind (`pool` for the current GAIE source)
- a short hash of the SPIFFE ID

That keeps names DNS-safe while allowing the SPIFFE ID to remain the real identity contract.

## Binding Status Exposure

`InferenceIdentityBinding.status.renderedClusterSPIFFEID` records the core
managed output after a successful reconcile:

| Field | Meaning |
| --- | --- |
| `name` | Deterministic managed `ClusterSPIFFEID` name. |
| `spiffeID` | Rendered SPIFFE ID, matching `status.computedSpiffeIDs`. |
| `selectorFingerprint` | `sha256:<hex>` fingerprint of the canonical selector set rendered into `spec.workloadSelectorTemplates`. |
| `observedGeneration` | Observed managed `ClusterSPIFFEID` generation when Kubernetes reports a persisted generation. Omitted when no persisted generation has been reported. |

Generic managed `ClusterSPIFFEID` read, create, update, or delete API failures
report `RenderFailure=True` with reason `ManagedOutputApplyFailed` and clear
`computedSpiffeIDs`, `renderedSelectors`, and `renderedClusterSPIFFEID` so
status-only clients do not read stale rendered output. These failures preserve
pending or confirmed ownership so retry and cleanup retain durable authority.

## Other Resources Touched

| Resource | Role |
| --- | --- |
| `InferenceIdentityBinding` | Source resource for managed output. |
| [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) | Required selector source resolved from `spec.poolRef.name`. |
| [`ClusterSPIFFEID`](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) | Managed output resource written by the reconciler. |

## Read And Watch Behavior

The controller:

- watches `InferenceIdentityBinding`
- watches supported `InferencePool` objects and requeues referencing bindings plus affected namespace peers
- requeues namespace peers for binding creation, update, and deletion so exclusivity converges without in-memory state
