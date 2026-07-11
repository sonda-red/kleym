---
title: Boundary Label Ownership
weight: 35
description: "Opt-in Kubernetes admission policy reference for platform-controlled Kleym identity boundary labels on Pods."
---

`identity.kleym.sonda.red/*` labels select a workload identity boundary. They
are security-sensitive: an actor that can choose one can select a corresponding
Kleym-managed SPIFFE identity.

Kleym does not enforce this ownership at the Pod API. It never creates or
mutates workloads, pools, or Pods. A platform must enforce ownership at
admission and grant the relevant workload permissions separately.

## Opt-In Admission Reference

[`boundary-label-ownership.yaml`][reference-manifest] is an opt-in,
cluster-scoped reference manifest. It uses the stable
`admissionregistration.k8s.io/v1` `ValidatingAdmissionPolicy` API, available in
Kubernetes v1.30 and later. It is not included in the default Kleym install or
the reference-environment kustomization.

The manifest treats the authenticator-provided group
`kleym.sonda.red/platform-workload-operators` as a concrete platform-controlled
actor group. Replace that group only with the group your authentication system
actually asserts for the platform workload operator.

Apply the policy and binding deliberately, after reviewing their cluster-wide
effect:

```sh
kubectl apply -f test/reference/inference-environment/boundary-label-ownership.yaml
```

The policy and its binding apply to Pod `CREATE` and `UPDATE` requests in every
namespace. They enforce these rules:

| Request | Result |
| --- | --- |
| Create a Pod with no reserved label | Allowed by this policy. |
| Create a Pod with a reserved label | Allowed only for a member of `kleym.sonda.red/platform-workload-operators`. |
| Add, change, or remove a reserved label on an existing Pod | Denied for every actor, including the platform group. |
| Change ordinary Pod metadata without touching a reserved label | Allowed by this policy; Kubernetes RBAC and other admission controls still apply. |

The CEL update validation compares the reserved-label subset of the old and new
Pod metadata in both directions. This rejects additions, value changes, and
removals while leaving non-reserved metadata updates outside this policy.

## Boundary Changes Use Replacement Pods

To change `identity.kleym.sonda.red/variant=prefill` to `decode`, do not mutate
the existing Pod. The allowed platform actor deletes or replaces the workload's
Pod through its normal workload-management path, then creates the replacement
Pod with `identity.kleym.sonda.red/variant=decode`. The policy authorizes the
reserved label at creation time and rejects any attempt to change the old Pod's
boundary in place.

This is a Pod-level control. A Deployment, Job, or other workload controller
must create its replacement Pods as an identity that belongs to the configured
platform group if its templates use the reserved prefix.

## Responsibility Boundaries

This reference policy owns only admission control for reserved Pod labels. It
does not grant permissions and does not govern any other identity input:

- Kubernetes RBAC and any additional platform admission controls authorize
  `InferenceIdentityBinding` create, update, and patch requests.
- Kubernetes RBAC and admission control authorize service-account assignment on
  Pods.
- Kleym remains read-only for workloads, pools, and Pods; it has no
  workload-mutation RBAC permissions.

Use the least privilege appropriate to the platform. In particular, do not
infer that the example actor group should receive broad binding-write or
service-account-assignment permission merely because it may create Pods with a
reserved boundary label.

## Removal

Remove the binding before the policy:

```sh
kubectl delete validatingadmissionpolicybinding kleym-reserved-identity-boundary-labels
kubectl delete validatingadmissionpolicy kleym-reserved-identity-boundary-labels
```

[reference-manifest]: https://github.com/sonda-red/kleym/blob/main/test/reference/inference-environment/boundary-label-ownership.yaml
