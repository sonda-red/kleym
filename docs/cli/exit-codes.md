---
title: Exit Codes
weight: 50
aliases:
  - /reference/exit-codes/
---

Exit codes distinguish clean inspections, detected identity issues, and fatal
CLI failures.

| Code | Meaning |
| --- | --- |
| `0` | Inspection succeeded and no error-severity findings exist. |
| `2` | Inspection succeeded and error-severity findings exist. |
| `3` | Binding lookup succeeded and the requested binding was not found. |
| `4` | Usage, connection, discovery, or permission failure prevented inspection. |
| `5` | Internal CLI or serialization failure. |

`--strict` treats warning-severity findings as exit code `2`.

When output can be serialized, code `3` should emit a `binding-not-found`
report. Fatal pre-read failures do not require a complete report.
