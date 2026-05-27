---
title: CLI Spec
weight: 20
---

`kleym` is a read-only inspection CLI for `InferenceIdentityBinding` state. It recomputes desired identity state, compares it with observed Kleym-managed `ClusterSPIFFEID` output, and reports findings from current Kubernetes API state.

The CLI does not reconcile, mutate resources, issue credentials, configure gateways, evaluate policy, or talk directly to SPIRE Server.

## Command Surface

```bash
kleym --version
```

Prints the linked CLI version string and exits without contacting Kubernetes. Released binaries report the release tag. Unreleased local builds default to `dev`.

```bash
kleym inspect binding <name> -n <namespace>
```

Inspects one `InferenceIdentityBinding` end to end.

Supported flags:

```bash
-n, --namespace
-o, --output text|json|yaml|markdown
--strict
--context
--kubeconfig
--timeout
```

Default namespace is `default`. Default output is `text`. `--timeout` must be greater than zero.

## Output Contract

JSON is the stable machine contract. YAML mirrors the same normalized report data and field names as JSON. Text and Markdown are human-oriented and may change between releases.

Automation must consume:

```bash
kleym inspect binding <name> -n <namespace> -o json
```

Text output leads with a summary. Markdown is for documentation and PR comments. Neither format may introduce inspection semantics absent from the canonical report data.

`kleym inspect binding` emits a `BindingInspectionReport` with four core sections:

1. `desired`: what Kleym would render from the inspected binding and resolved inputs.
2. `observed`: Kleym-managed `ClusterSPIFFEID` resources, drift, status, and eligible workloads when pod reads are available.
3. `findings`: typed issues or notable states discovered during inspection.
4. `capabilities`: whether each inspection area was complete, partial, skipped, or unknown.

Top-level JSON shape:

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "BindingInspectionReport",
  "generatedAt": "",
  "bindingRef": {},
  "resolvedInput": {},
  "desired": {},
  "observed": {},
  "findings": [],
  "capabilities": {}
}
```

Report fields and findings are documented in [Inspection Report][inspection-report-reference] and [Findings][findings-reference]. User-facing command guidance lives in [CLI Usage][cli-usage] and [CLI Results][cli-results].

## Inspect Binding Behavior

1. Resolve the binding, `poolRef`, and required or present `objectiveRef`; validate that an objective references the same pool.
2. Recompute desired identity state with shared Kleym logic.
3. Evaluate Kleym collision state from peer binding fingerprints when available; otherwise use the inspected binding's current `Conflict` condition and mark peer analysis partial or unknown.
4. Locate Kleym-managed `ClusterSPIFFEID` resources, compare desired and observed state, and evaluate eligible workloads when pod reads are available.
5. Emit the report and exit according to finding severity and inspection completeness.

`kleym` reports eligibility, not credential use. It reports deterministic Kleym collisions and visible drift; it does not fully simulate SPIRE selection behavior or analyze unrelated non-Kleym `ClusterSPIFFEID` overlap.

## Exit Behavior

1. `0`: inspection succeeded and no error-severity findings exist.
2. `2`: inspection succeeded and error-severity findings exist.
3. `3`: binding lookup succeeded and the requested binding was not found.
4. `4`: usage, connection, discovery, or permission failure prevented inspection.
5. `5`: internal CLI or serialization failure.

`--strict` treats warning-severity findings as exit code `2`. See [Exit Codes][exit-codes-reference].

## Implementation Boundary

The CLI and operator share pure logic for GAIE GVK discovery and resolution, pool selector derivation, selector rendering, SPIFFE ID rendering, deterministic `ClusterSPIFFEID` naming, and Kleym collision fingerprinting.

`kleym` does not import controller orchestration, finalizer handling, watches, status patching, or resource mutation logic.

## References

[cli-usage]: /cli/usage/
[cli-results]: /cli/results/
[inspection-report-reference]: /cli/inspection-report/
[findings-reference]: /cli/findings/
[exit-codes-reference]: /cli/exit-codes/
