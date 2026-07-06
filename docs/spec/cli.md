---
title: CLI Spec
weight: 20
description: "Authoritative CLI contract for kleym status, binding inspection, JSON output, findings, strict mode, and exit codes."
---

`kleym` is a read-only CLI for inspecting Kleym identity state. `kleym status` summarizes the visible cluster-level Kleym installation and binding health. `kleym inspect binding` renders Kleym identity output for one `InferenceIdentityBinding`, shows current binding conditions, reports Kubernetes-visible pod matches, and emits findings.

The CLI does not reconcile, mutate resources, compare live managed output, issue credentials, configure gateways, evaluate policy, or talk directly to SPIRE Server. Binding list behavior uses Kubernetes-native CRD printer columns through `kubectl get inferenceidentitybindings.kleym.sonda.red`.

## Command Surface

- `kleym --version`: print the linked version without contacting Kubernetes. Released binaries report the release tag; unreleased local builds default to `dev`.
- `kleym status`: summarize visible Kleym installation health, CRD availability, binding health, condition counts, and findings.
- `kleym inspect binding <name> -n <namespace>`: inspect one `InferenceIdentityBinding`.

Supported status flags: `-o, --output text|json`, `--context`, `--kubeconfig`, `--timeout`.

Supported inspect flags: `-n, --namespace`, `-o, --output text|json`, `--strict`, `--context`, `--kubeconfig`, `--timeout`, `--trust-domain`, `--clusterspiffeid-class-name`.

Defaults: namespace `default`, output `text`, trust domain `kleym.sonda.red`, classless `ClusterSPIFFEID` output. `--timeout` must be greater than zero. `--trust-domain` follows `kleym-operator --trust-domain` validation.

## Output Contract

JSON is the stable machine contract. Automation must use `kleym status -o json` or `kleym inspect binding <name> -n <namespace> -o json`. Text is the compact human view and may change between releases.

`KleymStatusReport` contains:

```json
{
  "schemaVersion": "v1alpha1",
  "kind": "KleymStatusReport",
  "generatedAt": "",
  "status": "",
  "cliVersion": "",
  "components": {},
  "config": {},
  "summary": {},
  "findings": []
}
```

Field meanings: `status` is the aggregate result (`OK`, `WARNING`, or `ERROR`); `cliVersion` records the linked CLI version; `components` records overall Kleym health, operator deployment details, Kleym API versions, SPIRE CRD versions, and Gateway API Inference Extension CRD versions; `config` records visible trust domain and `ClusterSPIFFEID` class configuration; `summary` records binding counts and condition counts; `findings` records status issues.

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

Field meanings: `identityConfig` records render config and sources; `bindingRef` records binding identity, refs, generation, and conditions; `resolvedInput` records resolved pool input; `renderedIdentity` records SPIFFE ID and selectors; `renderedClusterSPIFFEID` records deterministic managed output; `matchedPods` records readable matching pods or containers; `findings` records inspection issues.

Text output must use direct labels: `Identity`, `ClusterSPIFFEID`, `Conditions`, `Matched pods`, `Findings`, and `Exit code`. It must not use `eligible`, `bound`, `issued`, or `attested` for pod or identity state.

Status text output must group the report under `Kleym`, `Bindings`, and `Dependencies`. Component availability uses `Available`, `Warning`, `Unavailable`, and `unknown`.

## Status Behavior

1. Check whether required Kleym, SPIRE Controller Manager, and Gateway API Inference Extension CRDs are served, including visible served versions.
2. Discover the Kleym operator Deployment by the standard `app.kubernetes.io/name=kleym` and `app.kubernetes.io/component=operator` labels and report deployment name, ready replicas, total replicas, image tag version, and whether at least one replica is ready.
3. List visible `InferenceIdentityBinding` resources across namespaces.
4. Record visible trust domain and `ClusterSPIFFEID` class from binding status, with operator Deployment args as fallback when bindings are not available.
5. Count bindings as `OK` when `Ready=True`, `WARNING` when readiness is missing or unknown, and `ERROR` when `Ready=False`.
6. Count true `Ready`, `Conflict`, `InvalidRef`, `UnsafeSelector`, and `RenderFailure` binding conditions.
7. Report `Conflict` condition visibility from binding status. The CLI must not recompute peer conflicts.
8. Emit findings and exit according to finding severity.

Status does not compare live managed `ClusterSPIFFEID` output, prove SVID issuance, prove workload attestation, prove identity consumption, or perform request-time authorization checks.

## Inspect Binding Behavior

1. Resolve the binding and `poolRef`.
2. Choose identity config by precedence: explicit flag, binding status, then CLI default.
3. Record config values and sources. If binding status lacks operator config, add a warning finding.
4. Render identity and deterministic `ClusterSPIFFEID` output with shared Kleym logic.
5. Read pods when permitted and report pods or containers matching rendered Kubernetes-observable selectors.
6. Preserve current binding conditions. Conflict state comes from the inspected binding's `Conflict` condition, not peer re-analysis.
7. Emit the report and exit according to finding severity.

Matched pods are not proof of SVID issuance, workload attestation, identity consumption, or request-time authorization.

## Exit Behavior

| Code | Meaning |
| --- | --- |
| `0` | Inspection or status evaluation succeeded and no error-severity findings exist. |
| `2` | Inspection or status evaluation succeeded and error-severity findings exist. |
| `3` | Binding lookup succeeded and the requested binding was not found. |
| `4` | Usage, connection, discovery, or permission failure prevented inspection or status evaluation. |
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
