# Kleym Test Discipline Skill

## Purpose

Keep Kleym tests contract-driven and refactor-tolerant.

## Core Rule

Test behavior that must not break.

Do not test implementation shape, helper existence, or every invalid permutation.

## Test Ownership

Test behavior at the lowest layer that owns it.

Use higher-layer tests only for boundary behavior.

- Pure packages: detailed validation and deterministic computation.
- Controller: reconciliation reaction, status, cleanup, watches, managed output.
- CLI and inspection: user-visible reports, findings, JSON, and exit codes.
- Envtest: Kubernetes API behavior only.

## Keep Tests When They Protect

- A documented contract.
- A safety invariant.
- A rendered resource shape.
- A status condition or reason.
- A CLI output or exit code.
- A reconciliation boundary.
- A real regression.

## Remove or Shrink Tests When They

- Duplicate lower-layer coverage.
- Assert helper internals.
- Repeat permutations already covered elsewhere.
- Make refactors harder without protecting behavior.
- Exist only because the code was easy to test.

## Controller Rule

Do not retest pure logic through the controller.

Controller tests should prove what the controller does with a result or error.

Example:

Keep one boundary test for:

```text
invalid pool selector -> UnsafeSelector / InvalidPoolSelector
````

Do not keep a controller matrix for every invalid selector syntax case if `internal/gaie` owns that validation.

## Envtest Rule

Use envtest only for behavior that requires Kubernetes machinery.

Do not use envtest for pure rendering, sorting, parsing, or validation.

## Review Checklist

Before adding a test, answer:

1. What contract does this protect?
2. Which layer owns the behavior?
3. Is this already tested closer to that layer?
4. Would this catch a meaningful regression?

If the answer is unclear, do not add the test.

## Final Bias

Prefer a smaller suite with strong contract coverage over a large suite that freezes accidental structure.