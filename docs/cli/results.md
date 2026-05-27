---
title: Results
weight: 20
---

Inspection results compare desired identity state with visible cluster state and
record typed findings.

## Output Formats

| Format | Use it for |
| --- | --- |
| `text` | Default human-readable terminal output. It starts with status, finding count, drift count, eligible workload count, and inspection completeness. |
| `json` | Stable machine contract for automation. |

Automation should use:

```sh
kleym inspect binding <name> -n <namespace> -o json
```

## Status

Human output summarizes report status from finding severities:

| Status | Meaning |
| --- | --- |
| `OK` | No warning or error findings. |
| `Warning` | At least one warning finding and no error findings. |
| `Error` | At least one error finding. |

The status is an inspection result, not proof that a workload fetched or used an
SVID.

## Findings And Capabilities

Findings identify issues or notable states, such as invalid references,
deterministic Kleym collisions, observed drift, missing dependencies, or RBAC
limits. See [Findings](/cli/findings/) for the current finding classes.

Capabilities explain how complete each inspection area was. A check can be
`full`, `partial`, `skipped`, or `unknown`. RBAC limits and missing optional
resources should reduce capability instead of causing the CLI to guess.

## Exit Behavior

Exit code `0` means inspection completed without error-severity findings. Exit
code `2` means inspection completed and found at least one error-severity
finding. With `--strict`, warning-severity findings also return `2`.

See [Exit Codes](/cli/exit-codes/) for the complete process contract.
