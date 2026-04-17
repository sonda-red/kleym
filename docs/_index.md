---
title: kleym Documentation
layout: hextra-home
toc: false
sidebar:
  exclude: true
cascade:
  type: docs
---

{{< hextra/hero-section >}}
{{< hextra/hero-badge link="https://github.com/sonda-red/kleym/releases" >}}
Release stream: v1.x
{{< /hextra/hero-badge >}}

{{< hextra/hero-headline >}}
Deterministic SPIFFE identities for GAIE inference workloads
{{< /hextra/hero-headline >}}

{{< hextra/hero-subtitle >}}
`kleym` is a Kubernetes operator that compiles inference identity intent into deterministic `ClusterSPIFFEID` resources.
{{< /hextra/hero-subtitle >}}

{{< hextra/hero-button text="Install kleym" link="/install" >}}
{{< hextra/hero-button style="secondary" text="Read API reference" link="/reference/api" >}}
{{< /hextra/hero-section >}}

{{< cards cols="2" >}}
{{< card link="/concepts" title="Concepts" subtitle="Identity boundaries, selector safety, and scope." >}}
{{< card link="/architecture" title="Architecture" subtitle="End-to-end flow from binding to SPIRE state." >}}
{{< card link="/examples/basic-binding" title="Examples" subtitle="PoolOnly and PerObjective manifests with outcomes." >}}
{{< card link="/reference/api" title="Reference" subtitle="Stable API, condition, and resource facts." >}}
{{< card link="/spec" title="Authoritative Spec" subtitle="Contract for behavior, reconciliation, and safety." >}}
{{< card link="/contributing" title="Contributing" subtitle="Workflow, testing, and repository conventions." >}}
{{< /cards >}}
