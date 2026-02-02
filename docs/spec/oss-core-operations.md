# OSS Core Operations

## Observability

### Metrics

| Metric | Labels | Description |
|--------|--------|-------------|
| `terence_admission_denied_total` | `reason` | Admission denials by reason |
| `terence_inferenceworkload_ready_total` | | Ready workloads |
| `terence_dra_claims_created_total` | `deviceProfile` | DRA claims created |
| `terence_reconcile_duration_seconds` | `controller` | Reconciliation latency |

### Events

Emit Kubernetes Events for:
- `Denied` — admission rejection
- `Created` — workload created
- `Updated` — workload updated
- `Deleted` — workload deleted

### Logs

Structured logs must include:
- `namespace`
- `inferenceWorkload` name
- `modelDigest`
- `spiffeId`

---

## Security Boundaries

| Boundary | Enforcement |
|----------|-------------|
| No prompt/response storage | Controller never reads or stores inference payloads |
| No request-level auditing | Only deployment-level audit records (see Pro for request audit) |
| Least-privilege RBAC | Controller uses minimal permissions |
| No cross-namespace references | Webhook rejects any cross-namespace object references |

---

## Acceptance Criteria

OSS Core is complete when:

1. ✅ A tenant **cannot** deploy inference workloads without a bound profile
2. ✅ A tenant **cannot** request an unapproved DeviceProfile
3. ✅ A tenant **cannot** deploy a model without an immutable digest (strict mode)
4. ✅ A platform admin **can** query which `modelDigest` and `spiffeId` are running for every InferenceWorkload
5. ✅ Audit records exist for creates, updates, deletes, and denials

---

## Packaging

- **Delivery:** Helm chart + CRDs
- **License:** Apache 2.0 for code, separate trademark policy
- **Pro features:** Live in separate binaries (see [Pro Extensions](pro-extensions.md))
