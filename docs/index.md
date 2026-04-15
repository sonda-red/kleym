# kleym Documentation

`kleym` is a technical product repo with one authoritative behavior contract and a smaller set of supporting pages organized by reader intent.

Start here:

- Read the [spec](spec.md) if you need the contract for product behavior, API expectations, or controller outcomes.
- Read the [roadmap](roadmap.md) if you need current delivery phases and near-term priorities.
- Read [install](install.md) if you need to run the controller locally, deploy it to a cluster, or execute tests.
- Read the pages under [reference](reference/api.md) if you need stable facts about the current API surface, condition set, or managed resources.
- Read the pages under [design](design/reconciliation.md) if you need explanatory internals for reconciliation flow, selector safety, or collision handling.
- Read the pages under [examples](examples/basic-binding.md) if you need concrete YAML and the outcomes `kleym` is expected to produce.
- Read [contributing](contributing.md) if you are making changes in the repository.

Documentation rules for this repo:

- `spec.md` stays singular and authoritative.
- `reference/` states what is true today.
- `design/` explains why the controller works the way it does.
- `examples/` shows concrete manifests and expected outcomes.
- Issues and pull request text are design history, not documentation.
