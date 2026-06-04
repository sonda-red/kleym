---
title: Results
weight: 20
---

Status results summarize the visible Kleym installation, operator readiness,
identity configuration, required CRDs, binding condition counts, and typed
findings.

Inspection results show the identity and `ClusterSPIFFEID` Kleym renders for
one binding, current binding conditions, Kubernetes-visible matched pods, and
typed findings.

## Output Formats

| Format | Use it for |
| --- | --- |
| `text` | Default terminal output. Status uses grouped `Kleym`, `Bindings`, and `Dependencies` sections. Inspection uses sections for identity, `ClusterSPIFFEID`, conditions, matched pods, findings, and exit code. |
| `json` | Stable machine contract for automation. |

Automation should use:

```sh
kleym status -o json
kleym inspect binding <name> -n <namespace> -o json
```

Status text uses `Available`, `Warning`, `Unavailable`, and `unknown` for
component availability. It does not claim that SVIDs were issued, workloads
were attested, or identities were consumed.

## Findings

Findings identify issues or notable states, such as missing CRDs, unavailable
operator replicas, invalid references, deterministic Kleym collisions, missing
dependencies, unsupported selectors, or RBAC limits. See
[Findings](/cli/findings/) for the current finding classes.

## Exit Behavior

Exit code `0` means status or inspection completed without error-severity
findings. Exit code `2` means status or inspection completed and found at least
one error-severity finding. With `--strict`, warning-severity inspection
findings also return `2`.

See [Exit Codes](/cli/exit-codes/) for the complete process contract.
