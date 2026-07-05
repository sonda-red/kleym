---
title: Inference Workload Identity for Kubernetes
linkTitle: Inference Workload Identity
weight: 11
summary: A neutral concept reference for inference workload identity boundaries in Kubernetes model-serving pools.
description: Inference workload identity for Kubernetes defines stable workload identities from model-serving intent such as pools and objectives, rather than from pods alone.
aliases:
  - /inference-workload-identity/
---

Inference workload identity for Kubernetes is the practice of giving model-serving workloads stable identities that reflect inference intent, not only the pods that happen to run the serving code.

In this category, identity boundaries are derived from Kubernetes inference resources such as serving pools, model objectives, service accounts, namespaces, and workload selectors. The goal is to make an identity answer a question like "which pool or objective is this workload serving?" before downstream systems use that identity for registration, credential issuance, policy, routing, or audit.

This page is a category reference. It explains the broader problem before describing how Kleym currently maps that category onto SPIFFE and SPIRE Controller Manager.

## What inference workload identity means

A Kubernetes workload identity normally starts from Kubernetes workload facts: namespace, service account, labels, and container names. Inference workload identity adds model-serving intent to that boundary.

For model-serving systems, the relevant unit may be broader or narrower than one pod:

- a serving pool that can scale across many replicas
- one model objective served inside a shared pool
- one container inside a pod that hosts several serving processes
- one namespace and service account boundary that limits which workloads may receive the identity

The identity is useful when it is deterministic, scoped to an intended workload slice, and stable across normal pod replacement. It is unsafe when it can drift across tenants, collapse several objectives into one workload selection, or depend on mutable labels that do not represent the serving boundary.

## Why pod identity alone is not enough for model-serving pools

Pod identity is necessary, but it is often too low-level for inference serving.

Model-serving pools commonly reuse the same deployment shape for many model endpoints. Replicas change as traffic moves. One pod can serve more than one objective, and one objective can span many pods. If identity follows only the pod, the downstream system may not know whether it is looking at the pool, the objective, or an unrelated container in the same pod.

This creates two common failure modes:

- an identity is too broad, so multiple model-serving objectives share one security boundary
- an identity is too narrow or unstable, so ordinary pod churn produces new identities that do not match the intended serving boundary

Inference workload identity addresses that gap by deriving identity from serving intent and then proving that the rendered selectors still stay inside namespace and service-account boundaries.

## How InferencePool and InferenceObjective change the identity boundary

Gateway API Inference Extension (GAIE) resources describe inference serving intent in Kubernetes.

[`InferencePool`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferencepool/) represents a serving pool. For identity, it is the workload anchor: it names the pool and provides selector input for the pods that implement the pool.

[`InferenceObjective`](https://gateway-api-inference-extension.sigs.k8s.io/api-types/inferenceobjective/) represents objective-level serving intent associated with a pool. For identity, it can create a narrower boundary than the whole pool when several objectives share the same serving infrastructure.

Those resources do not issue credentials by themselves. They give an identity compiler enough intent to choose the intended boundary:

- pool identity: one identity for the serving pool
- objective identity: one identity for a specific objective within a pool
- container-scoped objective identity: one objective identity constrained to a named serving container when pod-level pool selectors would otherwise match too much

## How Kleym maps inference intent to SPIFFE / SPIRE Controller Manager

Kleym uses this category model as an identity-registration compiler.

An `InferenceIdentityBinding` declares which `InferencePool` is the workload anchor, which `InferenceObjective` is the optional objective boundary, which service account the workload must use, and whether the identity mode is `PoolOnly` or `PerObjective`.

`kleym-operator` resolves those inputs and renders deterministic SPIFFE IDs:

- `PoolOnly`: `spiffe://<trustDomain>/ns/<namespace>/pool/<pool-name>`
- `PerObjective`: `spiffe://<trustDomain>/ns/<namespace>/objective/<objective-name>`

It also renders selector sets that include the binding namespace, the service account, selectors derived from the referenced pool, and the container name for `PerObjective` mode. If Kleym cannot prove that the selectors remain within the intended boundary, it refuses to reconcile managed output.

Kleym materializes the result as SPIRE Controller Manager `ClusterSPIFFEID` resources. SPIRE Controller Manager then applies those resources to SPIRE registration state, and SPIRE remains responsible for identity issuance.

Kleym's current core behavior stops at identity registration. It does not deploy inference workloads, route traffic, configure gateways, evaluate request policy, issue credentials, or write SPIRE registration entries directly. Future extension work may define SVID consumption or validation artifacts separately, but that is not part of the current identity-registration contract.

## What this does not solve

Inference workload identity is not a complete inference security architecture by itself.

It does not decide which callers may invoke a model, which gateway routes a request, how traffic is scheduled, or whether a runtime has consumed an issued credential correctly. It does not prove request-time authorization, model ownership, data access, prompt safety, or audit semantics.

In Kleym specifically, a ready binding means the identity-registration intent was rendered and reconciled to the managed output boundary. It is not proof that an SVID was issued, that an application used the SVID, or that a gateway enforced policy with that identity.

## Where to go next

- Read [Kleym Concepts](/concepts/) for the project-specific identity modes, container-name boundary, and selector safety model.
- Read [Architecture](/architecture/) for the control flow from `InferenceIdentityBinding` to SPIRE Controller Manager output.
- Read [Operator Spec](/spec/operator/) for the authoritative current Kleym contract.
- Read [Selector Safety](/design/selector-safety/) and [Collision Detection](/design/collision-detection/) for the main safety invariants.
- Treat personal notes or lab writeups as discovery material. This page is the canonical Kleym docs reference for the neutral category term.
