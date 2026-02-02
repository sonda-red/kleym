# OSS Core API Reference

terence OSS Core exposes three CRDs and one audit record CRD.

## ClusterTenantProfile (cluster-scoped)

**Owner:** Platform team

### Spec

| Field | Type | Description |
|-------|------|-------------|
| `tenantSelector` | LabelSelector | Namespace label selector |
| `allowedModelSources` | []string | Allow-list of registries and repos |
| `requireImmutableModelRef` | bool | Require digest-pinned model references |
| `allowedDeviceProfiles` | []string | List of allowed DeviceProfile names |
| `quotas` | QuotaSpec | Limits for replicas and claims per namespace |
| `requiredIdentity` | bool | Require SPIFFE identity binding |
| `mode` | string | `strict` or `permissive` for tag resolution (default: `strict`) |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `boundNamespaces` | []string | Namespaces currently bound to this profile |
| `conditions` | []Condition | Standard Kubernetes conditions |

---

## DeviceProfile (cluster-scoped)

**Owner:** Platform team  
**Purpose:** Maps intent to DRA constructs

### Spec

| Field | Type | Description |
|-------|------|-------------|
| `deviceClassName` | string | Reference to DRA DeviceClass |
| `selectors` | []Selector | Driver attributes, optional CEL expressions |
| `allocationMode` | string | `exclusive` or driver-specific mode |
| `prioritizedAlternatives` | []string | Ordered list of fallback DeviceProfile names |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `resolvedDeviceClass` | string | Resolved DeviceClass name |
| `conditions` | []Condition | Standard Kubernetes conditions |

---

## InferenceWorkload (namespaced)

**Owner:** Tenant team  
**Purpose:** The only object tenants need to create

### Spec

| Field | Type | Description |
|-------|------|-------------|
| `modelRef` | string | Model reference (must be digest-pinned in strict mode) |
| `replicas` | int | Number of replicas |
| `deviceProfileRef` | string | Reference to DeviceProfile |
| `runtime` | string | Optional hint (does not affect guarantees) |
| `llmdAdapter` | bool | Enable llm-d adapter mode (default: `false`) |
| `service` | ServiceSpec | Port and protocol (default: ClusterIP) |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `effectiveProfileName` | string | Resolved ClusterTenantProfile |
| `resolvedModelDigest` | string | Immutable model digest |
| `spiffeId` | string | Assigned SPIFFE ID |
| `resourceClaimTemplateRefs` | []string | Generated DRA claim templates |
| `conditions` | []Condition | Standard Kubernetes conditions |

---

## InferenceAuditRecord (cluster-scoped, append-only)

**Owner:** terence controller  
**Purpose:** Immutable audit trail

### Spec

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | Time | Event timestamp |
| `namespace` | string | Source namespace |
| `inferenceWorkloadRef` | ObjectRef | Reference to InferenceWorkload |
| `modelDigest` | string | Model artifact digest |
| `spiffeId` | string | Workload SPIFFE ID |
| `deviceProfile` | string | DeviceProfile used |
| `action` | string | `created`, `updated`, `deleted`, or `denied` |
| `reason` | string | Human-readable reason |

### Immutability Rules

Records are created once and never updated. The controller should reject any update attempts.
