---
title: Results
weight: 20
---

Inspection results show the identity and `ClusterSPIFFEID` Kleym renders for
one binding, current binding conditions, Kubernetes-visible matched pods, and
typed findings.

## Output Formats

| Format | Use it for |
| --- | --- |
| `text` | Default terminal output. It uses compact sections for identity, `ClusterSPIFFEID`, conditions, matched pods, findings, and exit code. |
| `json` | Stable machine contract for automation. |

Automation should use:

```sh
kleym inspect binding <name> -n <namespace> -o json
```

## Findings

Findings identify issues or notable states, such as invalid references,
deterministic Kleym collisions, missing dependencies, unsupported selectors, or
RBAC limits. See [Findings](/cli/findings/) for the current finding classes.

## Exit Behavior

Exit code `0` means inspection completed without error-severity findings. Exit
code `2` means inspection completed and found at least one error-severity
finding. With `--strict`, warning-severity findings also return `2`.

See [Exit Codes](/cli/exit-codes/) for the complete process contract.
