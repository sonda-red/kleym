---
name: kleym-cli-change
description: Guide Kleym Cobra CLI changes. Trigger before editing CLI command behavior, flags, output formatting, inspection logic, JSON output, strict-mode behavior, or exit-code handling.
---

# Kleym CLI Change

Use this skill before modifying the `kleym` CLI. The CLI behavior is defined by `docs/spec/cli.md`.

## Required Context

- Read `docs/spec/cli.md` before changing CLI behavior.
- Read `docs/spec/operator.md` when CLI inspection depends on operator API or reconciliation behavior.
- Reuse existing pure render, resolve, selector, naming, and collision logic where available.
- Check directly relevant GitHub context when the task references an issue, PR, commit, or review comment, or when behavior is ambiguous from the specs.

## Implementation Rules

- Keep Cobra command handlers thin: parse flags, call inspection logic, format output, and return exit codes.
- Optimize CLI names and output labels for first-read human understanding. Prefer direct domain terms over internally derived terminology. JSON keys must stay stable, but new keys should be simple, explicit, and explainable without knowing reconciliation internals.
- Do not put identity derivation, selector safety, collision detection, GVK resolution, or rendering logic inside command files.
- Reuse the same pure render, resolve, selector, naming, and collision logic as the operator.
- Do not import controller reconciliation, watches, finalizers, status patching, or mutation paths into CLI code.
- Preserve deterministic, scriptable output.
- Preserve stable machine-readable JSON output for commands intended for CI or automation.
- Preserve the exit-code contract from `docs/spec/cli.md`, including `--strict`.
- Do not introduce interactive prompts or TUI behavior unless the issue explicitly asks for it.

## Verification

- Add focused tests for changed CLI behavior, output, inspection, or exit-code handling.
- Update `docs/spec/cli.md` for CLI command, output, inspection, or exit behavior changes.
- Update `README.md` only when overview, setup, or command examples change.
- Run `make test` for CLI behavior changes when applicable.
- Run `make lint` when touching Go code or build/CI logic.
