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

Kleym connects [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/) resources to SPIFFE workload identity for Kubernetes.

The in-cluster `kleym-operator` watches inference intent from resources such as [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) and [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/), then compiles that intent into deterministic SPIFFE identities and materializes them as SPIRE Controller Manager `ClusterSPIFFEID` resources. The companion `kleym` CLI is a read-only inspection tool for the rendered identity state.

For the broader category definition, read [Inference Workload Identity for Kubernetes](/concepts/inference-workload-identity/).

## Overview

- primary input: [`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/); optional objective subject: [`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/)
- primary output: deterministic `ClusterSPIFFEID` resources
- identity modes: `PoolOnly` and `PerObjective`
- safety model: namespace and service account selectors are always present; unsafe or ambiguous state is refused

## Documentation Map

### Operator docs

- [Install](/install/): local run, deployment, GitOps install, metrics, and validation commands
- [Concepts](/concepts/): GAIE inputs, identity modes, container discrimination, and selector safety
- [Inference Workload Identity](/concepts/inference-workload-identity/): neutral category reference for Kubernetes model-serving identity boundaries
- [Architecture](/architecture/): end-to-end reconcile flow from binding intent to SPIRE registration resources
- [Demo](/demo/): reference binding-to-`ClusterSPIFFEID` walkthrough
- [Examples](/examples/): concrete manifests and expected reconciliation outcomes
- [Reference](/reference/): API fields, conditions, managed resources, compatibility, dependencies, and GAIE compatibility
- [Troubleshooting](/troubleshooting/): binding conditions, missing CRDs, and collision triage
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
