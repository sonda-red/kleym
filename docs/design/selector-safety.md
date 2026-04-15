# Selector Safety

This page explains why selector rendering is intentionally narrow. The contract for what must be enforced lives in the [spec](../spec.md).

## Safety Goal

`kleym` should only write identities for workloads it can prove belong to the intended pool and namespace. The controller therefore intersects multiple selector sources instead of trusting a single input.

## Current Safety Layers

The current implementation requires:

- a namespace selector: `k8s:ns:<binding-namespace>`
- a service account selector: `k8s:sa:<service-account>`
- selectors derived from the referenced pool
- a container discriminator selector when `mode` is `PerObjective`

If the namespace selector does not match the binding namespace, reconciliation fails.

If no service account selector is present, reconciliation fails.

## Pool Selector Constraints

The controller currently accepts only deterministic pool selectors:

- `spec.selector.matchLabels`
- or a flat selector map that can be normalized into `matchLabels`

It rejects:

- empty selectors
- empty label keys or values
- `matchExpressions`
- pool selectors that cannot be decoded into a stable label map

That restriction is deliberate. `matchExpressions` are flexible, but they are harder to translate into precise SPIRE selector templates without widening the selected workload set.

## Why Templates Are Still Allowed

`workloadSelectorTemplates` remain useful for inserting stable, operator-controlled values such as namespace and service account selectors. They are not a substitute for pool-derived provenance; they are one half of the intersection.
