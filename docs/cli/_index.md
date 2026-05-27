---
title: CLI
weight: 20
---

`kleym` is the read-only inspection CLI. It recomputes the identity state that
`kleym-operator` would render, compares desired output with visible cluster
state, and serializes the result as a `BindingInspectionReport`.

Use this section for command usage, result interpretation, and the stable
machine-readable report contract.

| Page | Use it for |
| --- | --- |
| [Usage](/cli/usage/) | Building and running `kleym inspect binding`, including flags and RBAC expectations. |
| [Results](/cli/results/) | Choosing output formats and interpreting status, findings, drift, and capabilities. |
| [Inspection Report](/cli/inspection-report/) | Stable JSON/YAML report shape. |
| [Findings](/cli/findings/) | Finding IDs, severities, and meanings. |
| [Exit Codes](/cli/exit-codes/) | Process exit behavior for automation and strict mode. |

The authoritative command and output contract remains the [CLI Spec](/spec/cli/).
