---
title: CLI Spec
weight: 20
---

`kleym` is a read-only CLI for inspecting `InferenceIdentityBinding` state. `kleym inspect binding` renders Kleym identity output, shows current binding conditions, reports Kubernetes-visible pod matches, and emits findings.

The CLI does not reconcile, mutate resources, compare live managed output, issue credentials, configure gateways, evaluate policy, or talk directly to SPIRE Server. Cluster overview and list behavior belong to future `kleym status` and `kleym list` commands.

## Command Surface

- `kleym --version`: print the linked version without contacting Kubernetes. Released binaries report the release tag; unreleased local builds default to `dev`.
- `kleym inspect binding <name> -n <namespace>`: inspect one `InferenceIdentityBinding`.

Supported inspect flags: `-n, --namespace`, `-o, --output text|json`, `--strict`, `--context`, `--kubeconfig`, `--timeout`, `--trust-domain`, `--clusterspiffeid-class-name`.

Defaults: namespace `default`, output `text`, trust domain `kleym.sonda.red`, classless `ClusterSPIFFEID` output. `--timeout` must be greater than zero. `--trust-domain` follows `kleym-operator --trust-domain` validation.

## Output Contract

JSON is the stable machine contract. Automation must use `kleym inspect binding <name> -n <namespace> -o json`. Text is the compact human view and may change between releases.

`BindingInspectionReport` contains:

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "BindingInspectionReport",
  "generatedAt": "",
  "identityConfig": {},
  "bindingRef": {},
  "resolvedInput": {},
  "renderedIdentity": {},
  "renderedClusterSPIFFEID": {},
  "matchedPods": [],
  "findings": []
}
```

Field meanings: `identityConfig` records render config and sources; `bindingRef` records binding identity, refs, generation, mode, and conditions; `resolvedInput` records resolved pool/objective inputs; `renderedIdentity` records SPIFFE ID and selectors; `renderedClusterSPIFFEID` records deterministic managed output; `matchedPods` records readable matching pods or containers; `findings` records inspection issues.

Text output must use direct labels: `Rendered identity`, `Rendered ClusterSPIFFEID`, `Binding conditions`, `Matched pods`, `Findings`, and `Exit code`. It must not use `eligible`, `bound`, `issued`, or `attested` for pod or identity state.

## Inspect Binding Behavior

1. Resolve the binding, `poolRef`, and required or present `objectiveRef`.
2. Choose identity config by precedence: explicit flag, binding status, then CLI default.
3. Record config values and sources. If binding status lacks operator config, add a warning finding.
4. Render identity and deterministic `ClusterSPIFFEID` output with shared Kleym logic.
5. Read pods when permitted and report pods or containers matching rendered Kubernetes-observable selectors.
6. Preserve current binding conditions. Collision state comes from the inspected binding's `Conflict` condition, not peer re-analysis.
7. Emit the report and exit according to finding severity.

Matched pods are not proof of SVID issuance, workload attestation, identity consumption, or request-time authorization.

## Exit Behavior

| Code | Meaning |
| --- | --- |
| `0` | Inspection succeeded and no error-severity findings exist. |
| `2` | Inspection succeeded and error-severity findings exist. |
| `3` | Binding lookup succeeded and the requested binding was not found. |
| `4` | Usage, connection, discovery, or permission failure prevented inspection. |
| `5` | Internal CLI or serialization failure. |

`--strict` treats warning-severity findings as exit code `2`. See [Exit Codes][exit-codes-reference].

## Implementation Boundary

The CLI and operator share pure GAIE resolution, selector derivation, selector rendering, SPIFFE ID rendering, and deterministic `ClusterSPIFFEID` naming logic. `kleym` does not import controller orchestration, finalizer handling, watches, status patching, or resource mutation logic.

## References

[cli-usage]: /cli/usage/
[cli-results]: /cli/results/
[inspection-report-reference]: /cli/inspection-report/
[findings-reference]: /cli/findings/
[exit-codes-reference]: /cli/exit-codes/
