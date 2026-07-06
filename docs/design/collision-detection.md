---
title: Historical Collision Detection
weight: 30
description: "Historical note for removed PerObjective collision detection behavior."
aliases:
  - /operator/design/collision-detection/
---

`PerObjective` collision detection was part of the removed
`InferenceObjective` identity design. It is not part of current pool-only Kleym
reconciliation.

Current bindings render a single pool SPIFFE ID from `poolRef` and
`serviceAccountName`. The current controller does not resolve `objectiveRef`,
does not read `containerName`, and does not run per-objective collision checks.

The `Conflict` condition remains in status for compatibility with existing
reports, but current pool-only reconciliation normally sets it to `False` with
reason `Resolved`.
