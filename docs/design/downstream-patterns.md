---
title: Downstream Patterns
weight: 40
aliases:
  - /operator/design/downstream-patterns/
  - /reference/downstream-enforcement/
---

Non-normative. The [Operator Spec](/spec/operator/) remains the contract.

## Boundary

| Registration side | Downstream side |
| --- | --- |
| `InferenceIdentityBinding` API and status. | Inference workloads, routes, gateways, and serving behavior. |
| Managed `ClusterSPIFFEID` resources. | SPIRE installation, attestation, SVID issuance, and trust bundles. |
| Deterministic SPIFFE IDs and selectors. | Authentication, authorization, policy, and audit decisions. |

`Ready=True` means the binding reconciled. It is not proof that traffic is
authorized.

## Handoff Examples

| Pattern | Handoff |
| --- | --- |
| Envoy SDS | Envoy consumes SVID material from SPIRE Agent. |
| External authorization or OPA | A downstream policy service evaluates the SPIFFE ID. |
| JWT-SVID bridge | An external bridge maps a JWT-SVID to target-system credentials. |

`kleym-operator` does not configure these systems or render JWT-SVID-specific
`ClusterSPIFFEID` fields today.

## Runtime Flow

The usual downstream path is:

1. `kleym-operator` writes or updates a managed `ClusterSPIFFEID`.
2. SPIRE Controller Manager reconciles registration state into SPIRE.
3. SPIRE Agent attests matching workloads.
4. The workload or proxy receives an X.509-SVID or JWT-SVID.
5. A downstream system validates the SVID and applies policy.

The rendered SPIFFE ID is the stable join point. Downstream policy should key on
the SPIFFE ID and trust domain, not on internal Kleym object names, unless the
platform deliberately standardizes on that convention.

## Pattern Notes

| Pattern | Notes |
| --- | --- |
| Envoy SDS | Good fit when a proxy needs X.509-SVID material for TLS or mTLS. Envoy bootstrap, listeners, SDS socket mounts, validation context, and authorization filters remain external to Kleym. |
| Envoy Gateway external authorization | Good fit when the decision belongs at a route or gateway boundary. `SecurityPolicy`, the external authorization service, and route policy stay external to Kleym. |
| OPA | Good fit when a dedicated policy engine should evaluate SPIFFE ID plus request attributes such as route, method, tenant, or model metadata. |
| External processing | Good fit for adding audit metadata or normalizing authenticated request attributes. It should not invent workload identity. |
| JWT-SVID bridge | Good fit when a target system accepts OAuth or OIDC credentials. The bridge owns audience validation, subject mapping, token exchange, replay protection, and audit policy. |

## Integration Checklist

| Check | Expected result |
| --- | --- |
| Binding reconciles. | `InferenceIdentityBinding` reaches `Ready=True`. |
| Managed output exists. | `ClusterSPIFFEID` has the expected SPIFFE ID and selectors. |
| SPIRE materializes identity. | The matching workload receives the expected SVID. |
| Consumer authenticates identity. | Envoy, mesh, application, or bridge validates the SVID. |
| Policy enforces intent. | Gateway, OPA, external authorization, or application policy allows and denies expected requests. |
| Ownership remains clear. | External resources are documented as externally owned, not reconciled by Kleym. |
