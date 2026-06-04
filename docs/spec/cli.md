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
-o, --output text|json
--strict
--context
--kubeconfig
--timeout
--trust-domain
--clusterspiffeid-class-name
```

Default namespace is `default`. Default output is `text`. `--timeout` must be greater than zero.
`--trust-domain` must follow the same validation rules as `kleym-operator --trust-domain`.
`--clusterspiffeid-class-name` defaults to empty when used as a fallback or explicit override,
matching classless `ClusterSPIFFEID` output.

## Output Contract

JSON is the stable machine contract. Text is the human-oriented report and may change between releases.

Automation must consume:

```bash
kleym inspect binding <name> -n <namespace> -o json
```

Text output leads with a summary and must not introduce inspection semantics absent from the canonical report data.

`kleym inspect binding` emits a `BindingInspectionReport` with five core sections:

1. `identityConfig`: trust domain and class-name values used for desired rendering, plus their sources.
2. `desired`: what Kleym would render from the inspected binding and resolved inputs.
3. `observed`: Kleym-managed `ClusterSPIFFEID` resources, drift, status, and eligible workloads when pod reads are available.
4. `findings`: typed issues or notable states discovered during inspection.
5. `capabilities`: whether each inspection area was complete, partial, skipped, or unknown.

Top-level JSON shape:

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "BindingInspectionReport",
  "generatedAt": "",
  "identityConfig": {},
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
2. Choose identity config for recomputing desired output with this precedence for each field:
   explicit CLI flag, `InferenceIdentityBinding.status.trustDomain` and `status.clusterSPIFFEIDClassName`, then CLI default.
   The default trust domain fallback is `kleym.sonda.red`; the default class-name fallback is empty.
3. Record the chosen trust domain, class name, and per-field source in `report.identityConfig`.
   If binding status does not contain operator config, add a warning finding that inspection is using CLI defaults, flags, or a mix of both.
4. Recompute desired identity state with shared Kleym logic.
5. Evaluate Kleym collision state from peer binding fingerprints when available; otherwise use the inspected binding's current `Conflict` condition and mark peer analysis partial or unknown.
6. Locate Kleym-managed `ClusterSPIFFEID` resources, compare desired and observed state, and evaluate eligible workloads when pod reads are available.
7. Emit the report and exit according to finding severity and inspection completeness.

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
