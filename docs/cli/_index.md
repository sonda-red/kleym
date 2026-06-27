---
title: CLI
weight: 20
description: "Documentation for the read-only kleym CLI used to inspect installation status and rendered inference identity state."
---

`kleym` is the read-only inspection CLI. It renders Kleym identity output,
shows current binding conditions, reports Kubernetes-visible pod matches, and
serializes the result as a `BindingInspectionReport`.

Use this section for command usage, result interpretation, and the stable
machine-readable report contract.

| Page | Use it for |
| --- | --- |
| [Usage](/cli/usage/) | Building and running `kleym inspect binding`, including flags and RBAC expectations. |
| [Results](/cli/results/) | Choosing output formats and interpreting findings, matched pods, and exit behavior. |
| [Inspection Report](/cli/inspection-report/) | Stable JSON/YAML report shape. |
| [Findings](/cli/findings/) | Finding IDs, severities, and meanings. |
| [Exit Codes](/cli/exit-codes/) | Process exit behavior for automation and strict mode. |

The authoritative command and output contract remains the [CLI Spec](/spec/cli/).
