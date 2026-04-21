---
title: Compatibility Best Practice
weight: 55
---

## One Best Practice: Treat Compatibility as a Versioned Contract

If you need one rule that keeps `kleym` compatible with Gateway API, GAIE, llm-d, and SPIFFE/SPIRE, use this:

1. **Pin released API/component versions** (do not track `main`/unreleased heads).
2. **Test conformance at each layer** before promotion.
3. **Only use stable identity selectors** (namespace + service account + explicit container discriminator for model-level identity).

This single contract-first practice reduces drift between routing APIs (Gateway API + GAIE), inference stacks (llm-d), and workload identity issuance (SPIFFE/SPIRE).

## Why this is the right default

### Gateway API

Gateway API explicitly recommends using released bundles and notes that unreleased branches do not have compatibility guarantees.

- Versioning guide: <https://gateway-api.sigs.k8s.io/concepts/versioning/>
- Implementer guidance and conformance contract: <https://gateway-api.sigs.k8s.io/guides/implementers/>

### GAIE

GAIE conformance is defined across gateway implementations, inference routing extensions, and model server frameworks. GA migration guidance also calls out dual-version periods (`v1alpha2` + `v1`) where explicit GVR usage avoids ambiguity.

- Conformance: <https://gateway-api-inference-extension.sigs.k8s.io/concepts/conformance/>
- GA migration: <https://gateway-api-inference-extension.sigs.k8s.io/guides/ga-migration/>

### llm-d

llm-d infrastructure documentation currently requires Gateway API `v1.4.0+` and flags `kgateway` as a deprecated compatibility mode in favor of standalone `agentgateway`.

- llm-d infra prerequisites: <https://llm-d.ai/docs/architecture/Components/infra>

### SPIFFE/SPIRE

SPIRE workload identity mapping is selector-based. Operationally safe compatibility depends on deterministic, least-privilege selector sets and clear registration boundaries.

- SPIRE concepts (registration + selectors): <https://spiffe.io/docs/latest/spire-about/spire-concepts/>
- SPIRE Kubernetes configuration: <https://spiffe.io/docs/latest/deploying/configuring/>

## Practical policy for `kleym` operators

For cluster/platform operations, apply this policy to every upgrade:

1. **Pin versions**
   - Gateway API: install a released bundle only.
   - GAIE: pin release tag and know whether cluster is dual-version (`v1alpha2` + `v1`) during migration.
   - llm-d: pin chart/release and validate Gateway API floor (`v1.4.0+`).
   - SPIRE/SPIRE Controller Manager: pin release and CRD schema.

2. **Run compatibility gates in CI/staging**
   - Gateway API conformance profile used by your data plane.
   - GAIE conformance checks relevant to your deployment layer.
   - llm-d smoke tests for the selected gateway mode.
   - `kleym` reconcile stability checks (`make test`) against the pinned CRDs.

3. **Enforce identity selector discipline**
   - Keep namespace + service account safety selectors mandatory.
   - In `PerObjective` mode, require deterministic container discriminator.
   - Reject ambiguous or broad selectors rather than "best-effort" acceptance.

4. **Promote only as a set**
   - Upgrade and validate as a compatibility tuple, not one component at a time in production.

This keeps control-plane APIs, routing behavior, and identity issuance synchronized as one tested contract.
