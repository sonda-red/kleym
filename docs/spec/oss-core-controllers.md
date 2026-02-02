# OSS Core Controllers

## Reconcilers

### ClusterTenantProfile Reconciler

1. Watches namespaces and profiles
2. Computes profile binding based on `tenantSelector`
3. Optionally creates `ResourceQuota` objects for claim count and pod count enforcement
4. Updates `status.boundNamespaces`

### DeviceProfile Reconciler

1. Validates referenced DeviceClass exists
2. Validates selector syntax (CEL if used)
3. Updates `status.resolvedDeviceClass`

### InferenceWorkload Reconciler

1. **Resolve tenant profile** — Look up effective `ClusterTenantProfile` from namespace labels
2. **Validate modelRef** — Reject non-digest references in strict mode
3. **Create ServiceAccount** — Dedicated ServiceAccount for the workload
4. **Apply SPIFFE identity**
   - Annotate or create SPIRE registration resources based on integration mode
   - Record `spiffeId` in status
5. **Compile DRA**
   - Generate `ResourceClaimTemplate` from DeviceProfile
   - Attach `resourceClaims` to PodSpec
6. **Create workload resources**
   - Deployment + Service as the default
   - Optionally create llm-d resources if `llmdAdapter: true` (see adapter contract)
7. **Emit audit record** — Create `InferenceAuditRecord` for create and update
8. **Maintain status conditions**

---

## Admission Control

A validating webhook enforces the core guarantees at admission time.

### Validation Rules

| Rule | Description |
|------|-------------|
| Profile binding | Namespace must bind to exactly one `ClusterTenantProfile` |
| Model immutability | `modelRef` must be digest-pinned if strict mode is enabled |
| Device governance | `deviceProfileRef` must be in profile's `allowedDeviceProfiles` |
| Quota enforcement | `replicas` must be within quota limits |
| Adapter allowance | `llmdAdapter` must be allowed by platform if enabled |

### Mutating Rules

None in OSS Core by default.

If tag resolution is ever allowed:
- Must be explicit in profile `mode: permissive`
- Must lock the resolved digest into `status.resolvedModelDigest`
- Must set immutable annotation on the workload

---

## llm-d Adapter Contract

The adapter exists only to place identity, model immutability, and DRA governance into llm-d managed resources.

### Rules

1. **terence does not implement:**
   - Routing
   - Endpoint picking
   - Scheduling

2. **terence may generate or patch llm-d resources only to apply:**
   - ServiceAccount and identity wiring
   - ResourceClaims and device constraints
   - Labels and annotations containing `modelDigest`, `tenant`, `spiffeId`

### Integration Pattern

```
InferenceWorkload (llmdAdapter: true)
         │
         ▼
   terence controller
         │
         ├── Creates ServiceAccount with SPIFFE annotations
         ├── Creates ResourceClaimTemplate from DeviceProfile
         └── Patches llm-d resources with:
               • terence.sonda.red/model-digest
               • terence.sonda.red/spiffe-id
               • terence.sonda.red/tenant
               • resourceClaims reference
```

terence does not own llm-d routing, scheduling, or model runtime choices.
