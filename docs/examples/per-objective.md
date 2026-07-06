---
title: Historical PerObjective
weight: 20
description: "Historical note for removed PerObjective InferenceObjective examples; current Kleym examples use InferencePool only."
aliases:
  - /operator/examples/per-objective/
---

`PerObjective` examples are historical and are not a current Kleym contract.

Upstream Gateway API Inference Extension removed `InferenceObjective`, and Kleym
now derives GAIE identity input from `InferencePool` only. Current
`InferenceIdentityBinding` specs use `poolRef` and `serviceAccountName`; they do
not support `objectiveRef`, `mode`, or `containerName`.

Use [Basic Binding](/examples/basic-binding/) for the supported pool identity
example.
