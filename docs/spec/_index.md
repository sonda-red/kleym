---
title: Specs
weight: 80
description: "Authoritative Kleym product contracts for operator reconciliation behavior and read-only CLI inspection behavior."
---

Kleym keeps separate specs for the operator and CLI so CLI work can reference
the CLI contract without changing the operator contract by accident.

| Spec | Scope |
| --- | --- |
| [Operator Spec](/spec/operator/) | Authoritative product, API, and reconciliation behavior for `kleym-operator`. |
| [CLI Spec](/spec/cli/) | Read-only inspection CLI contract for `kleym`. |
