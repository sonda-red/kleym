---
name: kleym-controller-change
description: Guide Kleym operator API, generated manifest, internal/controller, and reconciliation changes. Trigger before editing API types, RBAC markers, generated manifests, controller watches, render logic, status conditions, identity derivation, selectors, finalizers, or reconciliation behavior.
---

# Kleym Controller Change

Use this skill before changing operator API shape or reconciliation behavior. Preserve correctness, idempotency, deterministic identity, and multi-tenant safety over convenience.

## Required Context

Read the relevant sections before editing:

- `docs/spec/operator.md` for operator product, API, and reconciliation contract.
- `docs/spec/cli.md` if CLI inspection depends on the changed operator behavior.
- Directly relevant GitHub issues, PRs, or review threads when behavior is ambiguous or the task references them.
- Existing helpers, types, tests, and local controller patterns before adding code.

## Reconcile Shape

Keep reconciliation easy to explain and consistent:

1. fetch object
2. handle deletion
3. ensure finalizer
4. compute desired state from current inputs
5. apply child resources
6. patch status once near the end

Do not spread status mutations across helper functions unless unavoidable.

## Guardrails

- Preserve human ownership of CRD semantics, reconciliation behavior, identity derivation rules, and failure behavior.
- Set or refresh all known conditions every reconcile pass. Use `observedGeneration`.
- Prefer pure helper functions for render, validation, and collision detection.
- Keep side effects in a narrow apply phase.
- Preserve idempotency. A second reconcile with unchanged inputs must produce no object drift.
- Preserve multi-tenant safety. Never widen selectors beyond what can be proven from namespace, service account, and validated pool-derived inputs.
- Refuse ambiguous or unsafe state with an explicit condition reason and message. Do not guess.
- When adding watches or map functions, avoid namespace-wide fanout if an index or predicate can narrow it.
- For generated `ClusterSPIFFEID` resources, preserve deterministic naming and cleanup behavior.

## Required Checks

For reconciliation behavior changes, check whether each item applies:

- Update `docs/spec/operator.md` if operator API or reconciliation behavior changed.
- Update `docs/spec/cli.md` if CLI inspection behavior depends on the operator change.
- Run generators when API types, RBAC markers, or generated manifests changed.
- Add or update controller tests for happy path, invalid ref, unsafe selector, conflict, and resync stability as applicable.
- Verify delete and cleanup behavior remains idempotent.
- Run `make test` for API and controller changes.
- Run `make lint` when touching Go code or build/CI logic.

## Stop Conditions

Stop and report the gap instead of filling it with plausible code when uncertainty affects API shape, CRD semantics, selector safety, identity derivation, finalizer behavior, security boundaries, or failure behavior.
