---
title: Selector Safety
weight: 20
---

This page explains why selector rendering is intentionally narrow. The contract for what must be enforced lives in the [Operator Spec](/spec/operator/).

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

The flat selector form is accepted because GAIE resources are read through `unstructured` objects and schema shape can vary across served versions. Normalizing either shape into `matchLabels` keeps selector rendering deterministic while preserving compatibility.

It rejects:

- empty selectors
- empty label keys or values
- label keys or values that do not satisfy Kubernetes label syntax
- label values with leading or trailing whitespace
- `matchExpressions`
- pool selectors that cannot be decoded into a stable label map

Those restrictions are deliberate. The controller renders pool labels directly into SPIRE workload selectors, so it rejects malformed label input instead of normalizing it into a selector the pool did not specify. `matchExpressions` are flexible, but they are harder to translate into precise SPIRE selector templates without widening the selected workload set.

## Why Templates Are Still Allowed

`workloadSelectorTemplates` remain useful for inserting stable, operator-controlled values such as namespace and service account selectors. They are not a substitute for pool-derived provenance; they are one half of the intersection.
