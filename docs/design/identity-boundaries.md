---
title: Identity Boundaries
weight: 15
description: "Design rationale for service-account-scoped inference target identities across namespaces, service accounts, and inference pools."
aliases:
  - /operator/design/identity-boundaries/
---

## Boundaries

One SPIFFE identity represents workloads at the intersection of a required
service account and a resolved inference target.

The current `InferencePool` source resolves to identity anchor `pool/<pool-name>`.
Kleym combines that anchor with the binding namespace and service account in the
SPIFFE ID, and enforces the same namespace and service account through mandatory
workload selectors.

Raw source GVK data and binding names remain provenance. They do not define the
workload principal and do not enter SPIFFE ID paths.
