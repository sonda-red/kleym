# Kleym CLI Spec

`kleym` is a read-only inspection CLI for `InferenceIdentityBinding` state. It explains how one binding becomes a deterministic SPIFFE identity registration by recomputing desired state with the same pure render, resolve, selector, naming, and collision logic as `kleym-operator`; comparing that desired state with observed Kleym-managed `ClusterSPIFFEID` output; and reporting findings from current Kubernetes API state.

The CLI stays inside Kleym's identity registration boundary. It does not reconcile, mutate, issue credentials, configure gateways, evaluate policy, or talk directly to SPIRE Server.

The current controller implementation is `kleym-operator`. The CLI implementation is `kleym`.

## Dependencies

`kleym` requires Kubernetes API access, Kleym CRDs, served GAIE `InferencePool` and optional `InferenceObjective` CRDs, and the SPIRE Controller Manager `ClusterSPIFFEID` CRD. Pod read access is required only for eligible workload reporting. Direct SPIRE Server access is not required.

## Operator Contract References

`kleym` must follow the [Operator Spec][operator-spec], [API Reference][api-reference], [Managed Resources][managed-resources], [Conditions Reference][conditions-reference], and [Collision Detection][collision-detection] contracts, including the same served GVK discovery model as `kleym-operator`.

## CLI Surface

```bash
kleym inspect binding <name> -n <namespace>
```

The command inspects one `InferenceIdentityBinding` end to end.

```bash
-n, --namespace
-o, --output text|json
--strict
--context
--kubeconfig
--timeout
```

Default output is `text`. Stable machine output is `json`.

## Output Contract

JSON is the stable contract. Text is human-oriented and may change between releases.

`kleym inspect binding` emits a `BindingInspectionReport` with four core sections:

1. `desired`: what Kleym would render from the inspected binding and resolved inputs.
2. `observed`: Kleym-managed `ClusterSPIFFEID` resources, drift, status, and eligible workloads when pod reads are available.
3. `findings`: typed issues or notable states discovered during inspection.
4. `capabilities`: whether each inspection area was complete, partial, skipped, or unknown.

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

Core fields:

1. `bindingRef`: namespace, name, generation, mode, `poolRef`, `objectiveRef`, and current conditions.
2. `resolvedInput`: resolved pool, resolved objective when present or required, served GVKs, pool selector, container discriminator, and selector provenance.
3. `desired`: deterministic `ClusterSPIFFEID` name, SPIFFE ID, pod selector, workload selectors, selector provenance, hint, and fallback value, computed with shared Kleym logic.
4. `observed`: Kleym-managed `ClusterSPIFFEID` resources, rendered spec fields, status fields, desired-versus-observed drift, and eligible workloads when pod reads are available.

Workload eligibility means a pod or container currently matches the rendered identity selectors. It is a selector result, not proof that an application fetched, loaded, or used an SVID. In `PerObjective` mode, the container discriminator narrows an otherwise eligible pod to a specific container name or image.

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

Finding semantics:

1. `binding-not-found` is an error when the API lookup succeeds and the requested binding is absent.
2. `dependency-missing` is an error only when the missing dependency is required to inspect the requested binding; missing optional checks belong in `capabilities`.
3. `rbac-limited` reports a non-fatal permission limit after the binding has been read; permission failure before binding read is fatal.
4. `zero-eligible-workloads` is informational by default because scale-to-zero can be valid.
5. `ambiguous-container-match` is a warning from live pod inspection when a `PerObjective` discriminator maps to more than one container in an otherwise eligible pod, most commonly with `ContainerImage`.

`capabilities` records check completeness:

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

## Inspection Boundaries

1. `kleym` reports eligibility, not credential use.
2. `kleym` reports deterministic Kleym collisions and visible drift. It does not fully simulate SPIRE selection behavior or analyze unrelated non-Kleym `ClusterSPIFFEID` overlap.
3. Event correlation, external `ClusterSPIFFEID` overlap analysis, federation hints, and JWT audience hints are outside the core CLI contract.

## Inspect Binding Behavior

1. Resolve the binding, `poolRef`, and required or present `objectiveRef`; validate that an objective references the same pool.
2. Recompute desired identity state with shared Kleym logic.
3. Evaluate Kleym collision state from peer binding fingerprints when available; otherwise use only the inspected binding's current `Conflict` condition and mark peer analysis partial or unknown.
4. Locate Kleym-managed `ClusterSPIFFEID` resources, compare desired and observed state, and evaluate eligible workloads when pod reads are available.
5. Emit the report and exit according to finding severity and inspection completeness.

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

## References

[operator-spec]: operator.md
[api-reference]: ../reference/api.md
[managed-resources]: ../reference/resources.md
[conditions-reference]: ../reference/conditions.md
[collision-detection]: ../design/collision-detection.md
