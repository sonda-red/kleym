---
title: About
toc: false
type: docs
summary: Project scope, intent, and entry points for kleym.
description: kleym compiles inference identity intent into deterministic SPIFFE identities for GAIE-aligned inference workloads.
sidebar:
  exclude: true
  hide: true
---

`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic SPIFFE identities for GAIE-aligned inference workloads.

It exists to make workload identity legible, repeatable, and safe across inference stacks. Instead of treating SPIFFE registration as manual cluster glue, `kleym` derives stable `ClusterSPIFFEID` resources from the same namespaced objects operators already use to describe inference intent.

The project is intentionally narrow:

- it translates intent into identity registration
- it validates selector safety and refuses ambiguous or unsafe state
- it does not deploy workloads, route traffic, or evaluate request policy

If you need behavior details, start with the [spec](/spec). If you need stable facts about fields, conditions, and managed resources, use the [reference](/reference/api). If you need implementation rationale, read the [design notes](/design/).

Project links:

- [GitHub repository](https://github.com/sonda-red/kleym)
- [Release stream](https://github.com/sonda-red/kleym/releases)
- [Contributing guide](/contributing)
