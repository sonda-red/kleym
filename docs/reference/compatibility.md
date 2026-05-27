---
title: Compatibility
weight: 50
aliases:
  - /operator/reference/compatibility/
---

This page records the compatibility surfaces that matter when installing,
upgrading, or changing Kleym. The [Operator Spec](/spec/operator/) remains the
behavior contract.

## Current Baseline

| Surface | Current source of truth |
| --- | --- |
| Go toolchain | `go.mod`, README, and [Install](/install/) |
| Kubernetes libraries | `go.mod` |
| GAIE inputs | [GAIE Compatibility](/reference/gaie-compatibility/) |
| `ClusterSPIFFEID` output | [Managed Resources](/reference/resources/) |
| Runtime dependencies | [Dependencies](/reference/dependencies/) |
| Conditions | [Conditions](/reference/conditions/) |

The `main` branch is the development baseline until a versioned release is
published. Released installs should pin the manifest ref and controller image tag
to the same release version.

## Compatibility Policy

Kleym supports only the documented input and output surface:

- supported GAIE `InferencePool` and `InferenceObjective` GVKs and consumed
  fields
- Kleym-owned `InferenceIdentityBinding` API fields and status conditions
- rendered `ClusterSPIFFEID` fields documented in Managed Resources

Inference workloads, schedulers, routes, gateways, SPIRE installation details,
and policy systems are external compatibility surfaces. Validate those with the
owning project or platform.

## Change Checklist

| Change | Update | Validate |
| --- | --- | --- |
| Go, Kubernetes, controller-runtime, build, or CI dependency | README or install docs when the public floor changes | `make test`, `make lint` |
| GAIE GVK or consumed field | GAIE compatibility, API reference, operator spec, troubleshooting | resolver and partial-CRD tests |
| Rendered `ClusterSPIFFEID` field | Managed resources and operator spec | create, update, delete, and resync tests |
| Reconciliation or status behavior | Operator spec, conditions, troubleshooting | `make test`; e2e when cluster behavior changes |
| Docs-only compatibility guidance | This page and linked reference page | `make docs-build` |

## Upgrade Checks

Before promoting a dependency or external API upgrade:

1. Pin component versions for Kubernetes libraries, GAIE CRDs, SPIRE Controller
   Manager, and SPIRE.
2. Confirm required CRDs are served in the target cluster.
3. Apply a representative `InferenceIdentityBinding`.
4. Confirm the binding reaches `Ready=True`.
5. Inspect managed `ClusterSPIFFEID` output for expected SPIFFE IDs, selectors,
   labels, and rendered fields.
6. Validate downstream identity consumption outside Kleym.
