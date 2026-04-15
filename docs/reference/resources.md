# Resources

This page records the Kubernetes resources the current controller reads, watches, and writes.

## Resource Roles

| Resource | Role |
| --- | --- |
| `InferenceIdentityBinding` | Primary namespaced API owned by `kleym`. |
| `InferenceObjective` | Target object resolved from `spec.targetRef.name`. |
| `InferencePool` | Selector source resolved from the objective's `spec.poolRef`. |
| `ClusterSPIFFEID` | Managed output resource written by the reconciler. |

## Read And Watch Behavior

The controller:

- watches `InferenceIdentityBinding`
- watches supported `InferenceObjective` objects and maps them back to matching bindings
- watches supported `InferencePool` objects and maps them back to bindings whose objectives reference those pools

## Managed Output

Each managed `ClusterSPIFFEID` currently includes:

- `spec.spiffeIDTemplate`: the fully rendered SPIFFE ID
- `spec.podSelector`: the selector derived from the referenced pool
- `spec.workloadSelectorTemplates`: rendered safety selectors, pool-derived selectors, and the optional per-objective container selector

Managed `ClusterSPIFFEID` objects are labeled with:

- `kleym.sonda.red/managed-by=kleym`
- `kleym.sonda.red/binding-name=<binding-name>`
- `kleym.sonda.red/binding-namespace=<binding-namespace>`

The controller also uses the finalizer `kleym.sonda.red/inferenceidentitybinding-finalizer` to clean up managed `ClusterSPIFFEID` objects on deletion.

## Naming

Managed `ClusterSPIFFEID` names are deterministic and derived from:

- the `kleym` controller name
- binding namespace
- binding name
- rendered mode (`pool` or `objective`)
- a short hash of the SPIFFE ID

That keeps names DNS-safe while allowing the SPIFFE ID to remain the real identity contract.
