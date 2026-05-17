# Kleym CLI Spec

`kleym` is a read-only inspection CLI for `InferenceIdentityBinding` state.

It explains:

1. Desired identity computed with the same pure render, resolve, selector, naming, and collision logic as `kleym-operator`.
2. Observed Kleym-managed `ClusterSPIFFEID` output.
3. Findings and capability gaps visible from current Kubernetes API state.

Scope boundary: `kleym` is an inspection tool for Kleym identity registration state. It does not reconcile, mutate, issue credentials, configure gateways, evaluate policy, or talk directly to SPIRE Server.

The current controller implementation is `kleym-operator`. The CLI implementation is `kleym`.

## Core Problem

Kleym derives deterministic SPIFFE identity registrations from GAIE inference resources, but raw Kubernetes objects do not explain the derivation end to end. Operators need one report that separates desired identity, observed output, selector eligibility, findings, and incomplete checks.

## Core Value

1. Explain identity derivation without requiring users to read controller code.
2. Separate desired state, observed state, and findings in one report.
3. Reuse the same render, resolve, selector, naming, and collision logic as `kleym-operator`.
4. Provide stable JSON output for CI, support workflows, and future automation.
5. Preserve Kleym’s narrow boundary by inspecting identity registration state only.

## Dependencies

1. Kubernetes API access through kubeconfig or in-cluster config.
2. Kleym CRDs.
3. Gateway-API Inference Extension (GAIE) `InferencePool` and optional `InferenceObjective` CRDs served by the cluster.
4. SPIRE Controller Manager and its `ClusterSPIFFEID` CRD.

Pod read access is required to report eligible workloads.

Direct SPIRE Server access is not required.

## Operator Contract References

1. [Operator Spec][operator-spec] for scope, downstream pattern, GAIE signal, identity model, selector safety, and reconciliation behavior.
2. [API Reference][api-reference] for `InferenceIdentityBinding` fields and defaults.
3. [Managed Resources][managed-resources] for `ClusterSPIFFEID` labels, fields, naming, and ownership.
4. [Conditions Reference][conditions-reference] for condition types and reasons.
5. [Collision Detection][collision-detection] for per-objective collision fingerprinting and peer selection.

`kleym` must respect the same served GVK discovery model as `kleym-operator`.

## CLI Surface

Core command:

```bash
kleym inspect binding <name> -n <namespace>
```

The command inspects one `InferenceIdentityBinding` end to end.

Supported flags:

```bash
-n, --namespace
-o, --output text|json
--strict
--context
--kubeconfig
--timeout
```

Default output is `text`.

Stable machine output is `json`.

## Report Concepts

`kleym inspect binding` builds a `BindingInspectionReport` from the current Kubernetes API state.

The report separates four views:

1. Desired state: what Kleym would render from the inspected binding, resolved GAIE inputs, selector rules, and naming rules.
2. Observed state: Kleym-managed `ClusterSPIFFEID` resources and, when allowed, live pods that currently match the rendered selectors.
3. Findings: typed issues or notable states discovered during inspection.
4. Capabilities: whether each inspection area was complete, partial, skipped, or unknown.

Workload eligibility means a pod or container currently matches the rendered identity selectors. It is a selector result, not proof that an application fetched, loaded, or used an SVID.

Container eligibility applies only when container-level selection is relevant. In `PerObjective` mode, the container discriminator narrows an otherwise eligible pod to a specific container name or image. If the discriminator can match multiple containers in one pod, the report treats that as an ambiguous container match.

## Output Contract

Reports separate four sections:

1. Desired state.
2. Observed state.
3. Findings.
4. Capabilities.

JSON is the stable contract.

Text is human-oriented and may change between releases.

Top-level JSON shape:

```json
{
  "apiVersion": "cli.kleym.sonda.red/v1alpha1",
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

`bindingRef` identifies the inspected binding:

1. Namespace.
2. Name.
3. Generation.
4. Mode.
5. `poolRef`.
6. `objectiveRef`.
7. Current conditions.

`resolvedInput` explains the inference resources used to derive identity:

1. Resolved `InferencePool`.
2. Resolved `InferenceObjective`, when present or required.
3. Served GVK used for each resource.
4. Pool selector.
5. Container discriminator.
6. Namespace and service-account selector provenance.

`desired` explains what Kleym intends to render:

1. Deterministic `ClusterSPIFFEID` name.
2. SPIFFE ID.
3. Pod selector.
4. Workload selectors.
5. Selector provenance.
6. Hint.
7. Fallback value.

This section must be computed using shared Kleym logic.

`observed` explains what exists in the cluster:

1. Kleym-managed `ClusterSPIFFEID` resources for the binding.
2. Rendered `ClusterSPIFFEID` spec fields.
3. Status fields when available.
4. Difference between desired and observed state.
5. Eligible workloads when Pod reads are available.

`findings` contains typed issues:

```json
{
  "id": "",
  "severity": "info|warning|error",
  "reason": "",
  "message": "",
  "affectedRefs": []
}
```

Required finding classes:

1. `binding-not-found`
2. `invalid-ref`
3. `dependency-missing`
4. `unsafe-selector`
5. `render-failure`
6. `kleym-collision`
7. `zero-eligible-workloads`
8. `ambiguous-container-match`
9. `observed-drift`
10. `rbac-limited`

Condition-derived findings should preserve existing Kleym condition types and reasons where possible. See [Conditions Reference][conditions-reference].

`binding-not-found` is an error finding when the API lookup succeeds and the requested binding is absent.

`dependency-missing` is an error finding only when the missing dependency is required to inspect the requested binding. Missing optional checks should be represented through `capabilities`.

`rbac-limited` is a finding when RBAC prevents a non-fatal check after the binding has been read. Permission failure that prevents reading the binding is a fatal inspection failure.

`zero-eligible-workloads` is an informational finding by default because scale-to-zero can be valid.

`ambiguous-container-match` is a warning finding from live pod inspection. It applies when a `PerObjective` container discriminator maps to more than one container in an otherwise eligible pod, most commonly when `ContainerImage` is used.

`capabilities` explains which checks were complete, partial, skipped, or unknown.

Example:

```json
{
  "binding": "full",
  "gaieResources": "full",
  "clusterspiffeids": "full",
  "peerBindings": "partial",
  "pods": "skipped"
}
```

If RBAC or missing CRDs prevent a non-fatal check, `kleym` must report partial or unknown state instead of guessing.

If usage, connection, authentication, discovery, or permission failure prevents reading the requested binding at all, the command exits as a fatal failure and no complete inspection report is required.

## Inspection Constraints

Workload eligibility is not proof of credential use.

`kleym` may report that a pod or container is eligible for a rendered identity under current selectors. It must not claim that an application fetched, loaded, or used an SVID.

Multiple `ClusterSPIFFEID` resources can select the same pod set. This may be valid from SPIRE's perspective but unsafe for serving stacks that expect one identity. `kleym` reports deterministic Kleym collisions and visible drift. It does not fully simulate SPIRE selection behavior or analyze unrelated non-Kleym `ClusterSPIFFEID` overlap.

Event correlation, external `ClusterSPIFFEID` overlap analysis, federation hints, and JWT audience hints are outside the core CLI contract.

## Inspect Binding Behavior

1. Resolve the `InferenceIdentityBinding`.
2. Resolve `poolRef` to a supported `InferencePool` GVK.
3. Resolve `objectiveRef` when present or required by `PerObjective`.
4. Validate that the objective references the same pool when an objective is used.
5. Recompute desired identity state using shared Kleym logic.
6. Evaluate Kleym collision state by recomputing peer binding fingerprints when peer binding reads are available.
7. If peer binding reads are unavailable, derive collision findings only from the inspected binding's current `Conflict` condition and mark peer collision analysis partial or unknown.
8. Locate Kleym-managed `ClusterSPIFFEID` resources for the binding.
9. Compare desired state with observed rendered state.
10. Evaluate eligible workloads when Pod reads are available.
11. Emit findings.
12. Exit according to finding severity and inspection completeness.

## Exit Behavior

1. `0`: inspection succeeded and no error-severity findings exist.
2. `2`: inspection succeeded and error-severity findings exist.
3. `3`: binding lookup succeeded and the requested binding was not found. Emit a `binding-not-found` report when output can be serialized.
4. `4`: usage, connection, discovery, or permission failure prevented inspection.
5. `5`: internal CLI or serialization failure.

`--strict` treats warning-severity findings as exit code `2`.

## Implementation Boundary

The CLI and operator share pure logic for:

1. GAIE GVK discovery and resolution.
2. Pool selector derivation.
3. Selector rendering.
4. SPIFFE ID rendering.
5. Deterministic `ClusterSPIFFEID` naming.
6. Kleym collision fingerprinting.

`kleym` does not import controller orchestration, finalizer handling, watches, status patching, or resource mutation logic.

## Acceptance Criteria

1. `kleym inspect binding <name> -n <namespace>` explains one binding end to end.
2. Desired state, observed state, findings, and capabilities are clearly separated.
3. JSON output is versioned and stable.
4. Desired identity is computed with shared Kleym logic.
5. Observed `ClusterSPIFFEID` state is read from the cluster.
6. Desired versus observed drift emits a finding.
7. Eligible workloads are described as eligible, not as proven credential users.
8. Error-severity findings produce a non-zero exit code.
9. Tests cover ready binding, missing binding, invalid reference, unsafe selector, identity collision with full peer reads, identity collision with peer RBAC limits, RBAC-limited pod reads, zero eligible workloads, ambiguous container matches, and observed drift.
10. Documentation includes one healthy example and one unsafe example.

## References

[operator-spec]: operator.md
[api-reference]: ../reference/api.md
[managed-resources]: ../reference/resources.md
[conditions-reference]: ../reference/conditions.md
[collision-detection]: ../design/collision-detection.md
