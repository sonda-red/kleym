# Roadmap

This roadmap tracks delivery order for `kleym` and keeps scope aligned with the contract in [spec.md](spec.md).

## Phase 0: Public Readiness

1. Documentation site structure and navigation published.
2. CI hardening for public pull requests.
3. Repository hygiene baselines in place (`LICENSE`, `SECURITY.md`, contributor guidance).

## Phase 1: Contract Completeness

1. Finalize reconciliation condition semantics and reason codes.
2. Expand API and schema validation coverage around discriminator and selector constraints.
3. Ensure status surfaces rendered selectors and computed identities with deterministic formatting.

## Phase 2: Controller Robustness

1. Strengthen collision detection regression coverage across multi-objective pool-sharing scenarios.
2. Expand idempotency and invalid-reference reconciliation tests.
3. Validate `ClusterSPIFFEID` drift correction behavior under repeated resync.

## Phase 3: Operability

1. Publish stable install and upgrade flow for cluster operators.
2. Improve troubleshooting docs for common `InvalidRef`, `UnsafeSelector`, and `Conflict` states.
3. Define release and compatibility policy for future API evolution.

## Out of Scope

`kleym` remains an identity registration compiler. It does not manage inference deployment, routing, or policy enforcement concerns owned by other control planes.
