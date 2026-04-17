---
title: kleym Documentation
layout: hextra-home
toc: false
summary: Deterministic SPIFFE identities for GAIE inference workloads.
description: kleym compiles inference identity intent into deterministic ClusterSPIFFEID resources with validated selector safety and clear tenant boundaries.
sidebar:
  exclude: true
cascade:
  type: docs
---

<div class="kleym-home not-prose">
  <section class="kleym-home__hero">
    <div class="kleym-home__copy">
      <a class="kleym-home__badge" href="https://github.com/sonda-red/kleym/releases" target="_blank" rel="noreferrer">
        <span class="kleym-home__badge-dot"></span>
        <span>Release stream: v1.x</span>
      </a>
      <p class="kleym-home__eyebrow">Identity compiler for inference control planes</p>
      <h1 class="kleym-home__title">Deterministic SPIFFE identities for GAIE inference workloads</h1>
      <p class="kleym-home__lede"><code>kleym</code> compiles inference identity intent into stable <code>ClusterSPIFFEID</code> resources, preserving selector safety and tenant boundaries from the first reconcile.</p>
      <div class="kleym-home__actions">
        <a class="kleym-home__action kleym-home__action--primary" href="/install">Install kleym</a>
        <a class="kleym-home__action kleym-home__action--secondary" href="/reference/api">Read API reference</a>
      </div>
    </div>
    <div class="kleym-home__panel">
      <p class="kleym-home__panel-label">What the operator guarantees</p>
      <ul class="kleym-home__signal-list">
        <li class="kleym-home__signal">
          <span class="kleym-home__signal-title">Deterministic output</span>
          <span class="kleym-home__signal-copy">Stable SPIFFE ID templates and predictable managed resources derived from GAIE intent.</span>
        </li>
        <li class="kleym-home__signal">
          <span class="kleym-home__signal-title">Selector safety</span>
          <span class="kleym-home__signal-copy">Namespace and service account selectors remain mandatory, then get narrowed by validated pool state.</span>
        </li>
        <li class="kleym-home__signal">
          <span class="kleym-home__signal-title">Clear identity modes</span>
          <span class="kleym-home__signal-copy"><code>PoolOnly</code> and <code>PerObjective</code> let operators choose between serving-pool and model-endpoint identity boundaries.</span>
        </li>
      </ul>
    </div>
  </section>

  <section class="kleym-home__facts" aria-label="How kleym works">
    <article class="kleym-home__fact">
      <p class="kleym-home__fact-label">Input</p>
      <h2 class="kleym-home__fact-title">GAIE intent</h2>
      <p class="kleym-home__fact-copy">Read <code>InferenceObjective</code> and <code>InferencePool</code> metadata as the source of identity provenance.</p>
    </article>
    <article class="kleym-home__fact">
      <p class="kleym-home__fact-label">Compilation</p>
      <h2 class="kleym-home__fact-title">Validated selectors</h2>
      <p class="kleym-home__fact-copy">Intersect pool-derived state with mandatory safety selectors and optional container discrimination.</p>
    </article>
    <article class="kleym-home__fact">
      <p class="kleym-home__fact-label">Output</p>
      <h2 class="kleym-home__fact-title">Stable SPIRE resources</h2>
      <p class="kleym-home__fact-copy">Materialize deterministic <code>ClusterSPIFFEID</code> objects that stay stable across resyncs and cleanup.</p>
    </article>
  </section>

  <section class="kleym-home__section" aria-labelledby="docs-map-title">
    <div class="kleym-home__section-head">
      <p class="kleym-home__eyebrow">Documentation map</p>
      <h2 class="kleym-home__section-title" id="docs-map-title">Start in the right place</h2>
      <p class="kleym-home__section-copy">Use the spec for contract questions, the reference for stable facts, and the design notes when you need rationale.</p>
    </div>
    <div class="kleym-home__tiles">
      <a class="kleym-home__tile" href="/concepts">
        <span class="kleym-home__tile-title">Concepts</span>
        <span class="kleym-home__tile-copy">Identity boundaries, selector safety, and scope.</span>
      </a>
      <a class="kleym-home__tile" href="/architecture">
        <span class="kleym-home__tile-title">Architecture</span>
        <span class="kleym-home__tile-copy">End-to-end flow from binding intent to SPIRE-managed state.</span>
      </a>
      <a class="kleym-home__tile" href="/examples/basic-binding">
        <span class="kleym-home__tile-title">Examples</span>
        <span class="kleym-home__tile-copy">Concrete PoolOnly and PerObjective manifests with expected outcomes.</span>
      </a>
      <a class="kleym-home__tile" href="/reference/api">
        <span class="kleym-home__tile-title">Reference</span>
        <span class="kleym-home__tile-copy">Stable API, condition, and managed resource facts.</span>
      </a>
      <a class="kleym-home__tile" href="/spec">
        <span class="kleym-home__tile-title">Authoritative Spec</span>
        <span class="kleym-home__tile-copy">The behavioral contract for reconciliation, safety, and system boundaries.</span>
      </a>
      <a class="kleym-home__tile" href="/contributing">
        <span class="kleym-home__tile-title">Contributing</span>
        <span class="kleym-home__tile-copy">Workflow, validation commands, and repository conventions.</span>
      </a>
    </div>
  </section>
</div>
