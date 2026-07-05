---
title: Design
weight: 70
description: "Design notes for Kleym reconciliation, identity boundaries, selector safety, collision detection, and downstream handoff patterns."
aliases:
  - /operator/design/
---

Controller design notes that explain reconciliation behavior, safety boundaries,
identity boundaries, and downstream consumption patterns.

- [Reconciliation](/design/reconciliation/): current controller flow.
- [Selector Safety](/design/selector-safety/): selector validation rationale.
- [Identity Boundaries](/design/identity-boundaries/): pool and objective identity modes.
- [Collision Detection](/design/collision-detection/): current per-objective collision logic.
- [Kleym Diff Proof Cases](/design/kleym-diff-proof-cases/): proof cases for a future offline semantic identity diff.
- [Downstream Patterns](/design/downstream-patterns/): non-normative gateway, proxy, and policy handoff patterns.
