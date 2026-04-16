# Copilot Instructions For kleym

Use these rules whenever you generate commit messages for this repository.

## Commit Format

- Follow Conventional Commits: `<type>(<scope>): <subject>`
- `scope` is optional, but preferred when it improves clarity.
- Write a specific, imperative subject line based on the staged diff.
- Avoid vague subjects like `update`, `changes`, `cleanup`, or `misc fixes`.

## Allowed Types

- `feat`
- `fix`
- `perf`
- `refactor`
- `revert`
- `docs`
- `style`
- `test`
- `build`
- `ci`
- `chore`

## Release Semantics

- `feat` triggers a **minor** release.
- `fix`, `perf`, `refactor`, and `revert` trigger a **patch** release.
- `docs`, `style`, `test`, `build`, `ci`, and `chore` trigger **no release**.
- Breaking changes must include a `BREAKING CHANGE:` footer and should use `!` in the header when appropriate.

## Preferred Scopes

Use these when they match the change:

- `api`
- `controller`
- `config`
- `deps`

## Source Of Truth

These rules must stay aligned with:

- [`SEMANTIC_VERSIONING.md`](../SEMANTIC_VERSIONING.md)
- [`.releaserc.json`](../.releaserc.json)
