---
title: Compatibility
weight: 55
---

This page records version support, external API assumptions, and upgrade checks.
The [spec](../spec) remains the behavior contract.

For Gateway API Inference Extension (GAIE), `kleym` compatibility is guaranteed
only for the objective and pool shapes listed below.

## Terms

| Term | Meaning |
| --- | --- |
| Supported | Intended to work for the documented input or output surface. |
| Tested | Exercised by project tests or an explicit upgrade check. |
| Pinned | Resolved to a release, module version, CRD schema, or install bundle before promotion. |
| External | Required by the deployment, but not implemented or guaranteed by `kleym`. |

## Release Matrix

The `main` row records the development baseline until the first versioned
release.

| `kleym` version | Status | Go | Kubernetes libraries | GAIE inputs | SPIRE Controller Manager output | Validation |
| --- | --- | --- | --- | --- | --- | --- |
| `main` | Development | `1.26+` | `k8s.io/api`, `apimachinery`, and `client-go` `v0.36.0`; `controller-runtime v0.24.0` | See [GAIE Inputs](#gaie-inputs). | See [ClusterSPIFFEID Output](#clusterspiffeid-output). | `make test`, `make lint`, `make docs-build` |

## Compatibility Surfaces

| Surface | `kleym` position | Source of truth |
| --- | --- | --- |
| Go | Root module defines the public toolchain floor. | `go.mod`, README, install docs |
| Kubernetes API clients | Follows `client-go`, `apimachinery`, and controller-runtime module versions. | `go.mod` |
| Gateway API | External; no route object is a direct `kleym` input. | Selected gateway/inference stack docs |
| GAIE | Supported only for documented objective and pool GVKs and fields. | [API](api), [spec](../spec) |
| Inference stack | External; workloads, schedulers, routes, and gateways stay outside `kleym`. | Selected stack release docs |
| SPIRE | External unless a rendered SPIRE Controller Manager field changes SPIRE registration behavior. | SPIRE release docs |
| SPIRE Controller Manager | Supported for the documented `ClusterSPIFFEID` output fields. | [Managed Resources](resources), `ClusterSPIFFEID` CRD schema |

## GAIE Inputs

| Object | Supported GVKs | Consumed fields |
| --- | --- | --- |
| `InferenceObjective` | `inference.networking.x-k8s.io/v1alpha2`; `inference.networking.k8s.io/v1` when served | `spec.poolRef` |
| `InferencePool` | `inference.networking.k8s.io/v1`; `inference.networking.x-k8s.io/v1alpha2` | `spec.selector.matchLabels`; flat string label maps are normalized for compatibility |

`InferencePool` selectors must render deterministically. Non-empty
`spec.selector.matchExpressions` are not supported.

When `spec.poolRef.group` is set, the controller constrains pool resolution to
that group using the supported `InferencePool` GVKs. Groups outside the
documented GAIE groups are refused with `InvalidRef=True` and reason
`UnsupportedPoolGroup`.

| Startup case | Behavior |
| --- | --- |
| Some supported GAIE GVKs are unavailable | Log and skip unavailable GVKs. |
| No supported objective or pool GVK is served | Startup fails. |
| A supported GVK is installed after startup | Restart the controller to register new watches. |
| Binding references a missing objective or pool | Reconcile fails with `InvalidRef`. |

## ClusterSPIFFEID Output

`kleym` currently renders only these `ClusterSPIFFEID` spec fields:

| Field | Status |
| --- | --- |
| `spec.spiffeIDTemplate` | Rendered. |
| `spec.podSelector` | Rendered from the referenced pool. |
| `spec.workloadSelectorTemplates` | Rendered safety selectors, pool-derived selectors, and optional container discriminator. |
| `spec.fallback` | Not rendered. Requires design decision. |
| `spec.hint` | Not rendered. Requires design decision. |
| JWT-SVID-related fields | Not rendered. Requires user story and SPIRE Controller Manager/SPIRE version gate. |

Managed `ClusterSPIFFEID` objects are labeled with:

| Label | Purpose |
| --- | --- |
| `kleym.sonda.red/managed-by=kleym` | Marks controller ownership. |
| `kleym.sonda.red/binding-name=<binding-name>` | Finds children for a binding. |
| `kleym.sonda.red/binding-namespace=<binding-namespace>` | Disambiguates namespaced bindings for a cluster-scoped output resource. |

Related design issue: [#109](https://github.com/sonda-red/kleym/issues/109).

## Change Checklist

| Change | Required updates | Required validation |
| --- | --- | --- |
| Go, Kubernetes, controller-runtime, build, or CI-sensitive dependency | README or install docs if the public floor changes | `make test`, `make lint` |
| GAIE GVK or consumed field | [API](api), [spec](../spec), `docs/troubleshooting.md` | Resolver and partial-CRD tests |
| Rendered `ClusterSPIFFEID` field | [Managed Resources](resources), this page, design issue when behavior changes | Create, update, delete, and resync tests |
| Reconciliation or status behavior | [spec](../spec), [Conditions](conditions), troubleshooting docs | `make test`; `make test-e2e` when cluster behavior changes |
| Docs-only compatibility policy | This page | `make docs-build` |

## Upgrade Checks

Before promoting a dependency or external API upgrade:

| Check | Expected result |
| --- | --- |
| Pin component versions. | Kubernetes libraries, GAIE bundle, inference stack, SPIRE, and SPIRE Controller Manager versions are known. |
| Verify served APIs. | Required GAIE resources and `clusterspiffeids.spire.spiffe.io` exist. |
| Apply a representative binding. | Status reaches `Ready=True`. |
| Inspect managed output. | SPIFFE IDs, selectors, labels, and `ClusterSPIFFEID` fields match this page. |
| Reconcile unchanged inputs again. | No managed-object drift. |
| Validate downstream consumers. | Gateway, mesh, and policy behavior is checked outside `kleym`. |

## References

- Gateway API versioning: <https://gateway-api.sigs.k8s.io/concepts/versioning/>
- Gateway API implementer guidance: <https://gateway-api.sigs.k8s.io/guides/implementers/>
- GAIE conformance: <https://gateway-api-inference-extension.sigs.k8s.io/concepts/conformance/>
- GAIE migration guide: <https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/>
- llm-d infrastructure docs: <https://llm-d.ai/docs/architecture/Components/infra>
- SPIRE concepts: <https://spiffe.io/docs/latest/spire-about/spire-concepts/>
- SPIRE configuration: <https://spiffe.io/docs/latest/deploying/configuring/>
- SPIRE Controller Manager `ClusterSPIFFEID` docs: <https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md>
