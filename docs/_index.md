---
title: Kleym
linkTitle: Introduction
toc: false
summary: Project scope, intent, and documentation map for kleym.
description: Kleym translates Gateway API Inference Extension (GAIE) inference intent into deterministic Secure Production Identity Framework for Everyone (SPIFFE) identities.
cascade:
  type: docs
aliases:
  - /operator/
---

<div class="kleym-about-mark">
  <img src="/images/sondrd-512.png" alt="Sonda Red square logo" width="180" height="180">
</div>

Kleym connects Gateway API Inference Extension (GAIE) [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) resources to SPIFFE workload identity for Kubernetes.

The in-cluster `kleym-operator` watches `InferencePool` workload intent, then compiles that intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources. The companion `kleym` CLI is a read-only inspection tool for the rendered identity state.

## Overview

- primary input: [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/)
- primary output: deterministic `ClusterSPIFFEID` resources
- safety model: namespace, service account, pool, and identity-boundary selectors are mandatory; peer bindings must be structurally exclusive
- fail-closed behavior: unsafe, conflicting, or duplicate claims retain no managed output, and output absence is confirmed before conflicts settle
- enforcement assumption: cluster admission controls reserved identity-boundary labels and keeps them immutable for each Pod lifetime

## Documentation Map

### Operator docs

- [Install](/install/): local run, deployment, GitOps install, metrics, and validation commands
- [Concepts](/concepts/): GAIE pool input, service-account-scoped inference target identity, and selector safety
- [Architecture](/architecture/): end-to-end reconcile flow from binding intent to SPIRE registration resources
- [Demo](/demo/): reference binding-to-`ClusterSPIFFEID` walkthrough
- [Examples](/examples/): concrete manifests and expected reconciliation outcomes
- [Reference](/reference/): API fields, conditions, managed resources, compatibility, dependencies, and GAIE compatibility
- [Troubleshooting](/troubleshooting/): binding conditions, missing CRDs, and selector triage
- [Design](/design/): controller design notes and downstream handoff patterns

### CLI docs

- [CLI](/cli/): read-only inspection usage, results, report shape, findings, and exit codes

### Reference and specs

- [Operator Spec](/spec/operator/): authoritative operator behavior and API contract
- [CLI Spec](/spec/cli/): authoritative read-only inspection CLI contract
- [Contributing](/contributing/): workflow, validation, and repository conventions

## Project Links

- [GitHub repository](https://github.com/sonda-red/kleym)
- [Release stream](https://github.com/sonda-red/kleym/releases)
- [Contributing guide](/contributing)
- [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
- [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
