---
title: Identity Boundaries
weight: 15
description: "Design rationale for fail-closed inference identity boundaries across namespaces, service accounts, pools, and label-defined workload variants."
aliases:
  - /operator/design/identity-boundaries/
---

## Boundaries

One SPIFFE identity represents workloads at the intersection of a required
service account, resolved inference pool, and label-defined workload variant.

The current `InferencePool` source resolves to identity anchor `pool/<pool-name>`.
Kleym combines that anchor with the binding namespace, service account, and
boundary label value in the SPIFFE ID. Mandatory workload selectors enforce the
namespace, service account, complete pool selector, and canonical boundary label.

For distinct SPIFFE IDs in one namespace and service account, Kleym accepts only
one structural exclusivity proof: equal boundary label keys with different
values. Reused values, different keys, and duplicate SPIFFE IDs conflict. The
controller withdraws managed output for conflicts and confirms absence before it
reports the settled conflict state.

Raw source GVK data and binding names remain provenance. They do not define the
workload principal and do not enter SPIFFE ID paths.
