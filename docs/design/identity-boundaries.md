---
title: Identity Boundaries
weight: 15
description: "Design rationale for Kleym identity boundaries across namespaces, service accounts, and inference pools."
aliases:
  - /operator/design/identity-boundaries/
---

## Boundaries

One SPIFFE identity represents the serving pool pods selected by the referenced
`InferencePool`.

The pool defines where inference runs. Kleym adds namespace and service-account
selectors so the identity remains tied to the binding namespace and the expected
workload service account.
