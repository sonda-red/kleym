---
title: About
toc: false
type: docs
summary: Project scope, intent, and documentation map for kleym.
description: kleym translates GAIE inference intent into deterministic SPIFFE identities and keeps a narrow scope around identity registration and selector provenance.
sidebar:
  exclude: true
  hide: true
---

<div class="kleym-about-mark">
  <img src="/images/sondrd-512.png" alt="Sonda Red square logo" width="180" height="180">
</div>

`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic SPIFFE identities for GAIE-aligned inference workloads.

It exists to make workload identity legible, repeatable, and safe across inference stacks. Instead of treating SPIFFE registration as manual cluster glue, `kleym` derives stable `ClusterSPIFFEID` resources from the same namespaced objects operators already use to describe inference intent.

It is an identity registration compiler. The project is intentionally narrow:

- it translates intent into identity registration
- it validates selector safety and refuses ambiguous or unsafe state
- it does not deploy workloads, route traffic, or evaluate request policy

## Overview

- primary inputs: `InferenceObjective` and `InferencePool`
- primary output: deterministic `ClusterSPIFFEID` resources
- identity modes: `PoolOnly` and `PerObjective`
- safety model: namespace and service account selectors are always present; unsafe or ambiguous state is refused

## Documentation Map

### Core docs

- [Spec](/spec): authoritative behavior and API contract
- [API reference](/reference/api): fields, conditions, and managed resources
- [Concepts](/concepts): identity boundaries, selector safety, and scope
- [Architecture](/architecture): controller flow from intent to SPIRE resources

### Use and contribute

- [Install](/install): local run, deploy, and test commands
- [Examples](/examples): concrete manifests and expected outcomes
- [Troubleshooting](/troubleshooting): condition-driven debugging
- [Contributing](/contributing): workflow, validation, and repository conventions

## Project Links

- [GitHub repository](https://github.com/sonda-red/kleym)
- [Release stream](https://github.com/sonda-red/kleym/releases)
- [Contributing guide](/contributing)
