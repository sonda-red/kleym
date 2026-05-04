---
title: Downstream Enforcement
weight: 60
---

This page explains how platform systems can use the SPIFFE identities that
`kleym` registers. It is non-normative guidance. The [spec](/spec/) remains
the product and API contract.

## Handoff Boundary

`kleym` makes inference identity intent visible to SPIRE. Runtime enforcement
happens downstream in a gateway, mesh, proxy, policy service, or application.

| `kleym` owns | External systems own |
| --- | --- |
| `InferenceIdentityBinding` API and status. | `InferenceObjective`, `InferencePool`, workloads, routes, and serving behavior. |
| Managed `ClusterSPIFFEID` resources. | SPIRE Controller Manager installation, SPIRE Server, SPIRE Agent, and workload attestation. |
| Deterministic SPIFFE IDs and selector sets. | X.509-SVID or JWT-SVID issuance, trust bundles, rotation, and validation. |
| Identity registration health. | Request authentication, authorization, audit, and policy decisions. |

Do not read `Ready=True` on an `InferenceIdentityBinding` as proof that traffic
is authorized. It means the binding reconciled and the managed registration
output is in place.

Following the spec boundary, `kleym` does not create or modify gateway, route,
policy, Envoy, OPA, OAuth, or OIDC resources.

## Runtime Flow

The usual path is:

1. `kleym` writes or updates a managed `ClusterSPIFFEID`.
2. SPIRE Controller Manager reconciles the registration into SPIRE.
3. SPIRE Agent attests matching workloads.
4. The workload or proxy receives an X.509-SVID or JWT-SVID.
5. A downstream system validates the SVID and applies policy.

The rendered SPIFFE ID is the stable join point. Policy systems should key on
the SPIFFE ID and trust domain, not on internal `kleym` object names, unless the
operator deliberately chooses that convention.

## Common Patterns

### Envoy SDS

Use Envoy SDS when a proxy needs X.509-SVID material for TLS or mTLS.

In this pattern:

1. `kleym` registers the SPIFFE ID.
2. SPIRE issues the X.509-SVID.
3. Envoy consumes the certificate and trust bundle from SPIRE Agent through SDS.
4. Envoy or another downstream system enforces policy after authentication.

The operator still owns Envoy bootstrap, listener configuration, SDS socket
mounts, validation context, and authorization filters.

### Envoy Gateway External Authorization

Use Envoy Gateway `SecurityPolicy` with external authorization when the
decision belongs at a Gateway API route or gateway boundary.

A typical handoff looks like:

1. `kleym` registers the identity for the inference workload.
2. Envoy or a gateway-adjacent component authenticates the relevant peer.
3. Envoy Gateway calls an HTTP or gRPC external authorization service through a
   `SecurityPolicy`.
4. The authorization service evaluates the SPIFFE ID and request context.

The `SecurityPolicy`, external authorization service, and route policy are
external to `kleym`.

### OPA

Use Open Policy Agent (OPA) when policy should be evaluated by a dedicated
policy engine.

OPA commonly sits behind Envoy external authorization. A policy can evaluate the
SPIFFE ID together with request attributes such as route, method, headers,
tenant, model name, or proxy-supplied metadata.

OPA owns the policy language, policy data, decision logs, and rollout process.
`kleym` only makes the identity deterministic and inspectable.

### External Processing

Use Envoy Gateway `EnvoyExtensionPolicy` and ext-proc for request or response
processing. Do not use ext-proc to invent workload identity.

Good fits include:

- adding audit metadata derived from an authenticated SPIFFE ID
- normalizing request attributes before authorization
- recording model or objective metadata for observability
- enriching a request for a downstream policy service

The identity source of truth remains the SVID and trust bundle issued by SPIRE.

### JWT-SVID To OAuth Or OIDC Bridge

Use an external bridge when the target system accepts OAuth or OIDC credentials
instead of SPIFFE identity material.

The bridge validates a JWT-SVID and maps or exchanges it into the credential the
target system understands. That bridge owns:

- JWT-SVID audience selection and validation
- SPIFFE ID to client or subject mapping
- token exchange or client assertion behavior
- OAuth/OIDC provider configuration
- replay protection, lifetime, and audit policy

`kleym` does not render JWT-SVID-specific `ClusterSPIFFEID` fields today. See
[Compatibility](/reference/compatibility/) for the current `ClusterSPIFFEID` output
contract.

## Integration Checklist

For demos and platform integrations, check the boundary in this order:

| Check | Expected result |
| --- | --- |
| Binding reconciles. | `InferenceIdentityBinding` reaches `Ready=True`. |
| Managed output exists. | `ClusterSPIFFEID` has the expected SPIFFE ID and selectors. |
| SPIRE materializes identity. | The matching workload receives the expected X.509-SVID or JWT-SVID. |
| Consumer authenticates identity. | Envoy, mesh, application, or bridge validates the SVID. |
| Policy enforces intent. | Gateway, OPA, external authorization service, or application policy allows and denies the expected requests. |
| Ownership remains clear. | External resources are documented as externally owned, not reconciled by `kleym`. |

## References

- SPIFFE using Envoy with SPIRE: <https://spiffe.io/docs/latest/microservices/envoy/>
- SPIFFE concepts for SVIDs and the Workload API: <https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/>
- SPIFFE OPA authorization with Envoy and X.509-SVIDs: <https://spiffe.io/docs/latest/microservices/envoy-opa/readme/>
- SPIFFE OPA authorization with Envoy and JWT-SVIDs: <https://spiffe.io/docs/latest/microservices/envoy-jwt-opa/readme/>
- SPIRE Controller Manager `ClusterSPIFFEID` CRD: <https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md>
- Envoy Gateway `SecurityPolicy`: <https://gateway.envoyproxy.io/v1.7/concepts/gateway_api_extensions/security-policy/>
- Envoy Gateway external authorization: <https://gateway.envoyproxy.io/v1.7/tasks/security/ext-auth/>
- Envoy Gateway external processing: <https://gateway.envoyproxy.io/v1.7/tasks/extensibility/ext-proc/>
- OPA Envoy plugin: <https://www.openpolicyagent.org/docs/envoy>
