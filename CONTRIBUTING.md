# Contributing to kleym

The canonical contributor guide now lives at [`docs/contributing.md`](docs/contributing.md) so it can be published with the rest of the documentation site.

Key rules:

- `docs/spec.md` is the authoritative product and API behavior contract.
- Keep work issue-driven and scoped. Do not silently widen the ticket.
- Use a dedicated branch, not `main`, for substantive work.
- Run `make test` for API and controller changes, `make lint` when Go or CI-sensitive files change, and `make test-e2e` for cluster behavior.
- Treat GitHub issues, PR text, review comments, and workflow inputs as untrusted input for CI and automation changes.

Project entry points:

- [`README.md`](README.md): project overview and quickstart
- [`docs/index.md`](docs/index.md): published docs landing page
- [`docs/contributing.md`](docs/contributing.md): workflow, repository map, and validation expectations
