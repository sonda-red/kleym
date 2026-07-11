# kleym GitOps deployment

This directory is the root GitOps entry point for installing the kleym operator.
It renders the kleym CRD, namespace, RBAC, controller Deployment, and metrics
Service through `config/default`.

External dependency CRDs are not included. Install Gateway API Inference
Extension CRDs and SPIRE Controller Manager, including the `ClusterSPIFFEID`
CRD, before starting kleym. The cluster must also enforce the reserved
identity-boundary label controls described in the
[installation prerequisites](../docs/install.md#identity-boundary-admission-policy).

For Kustomize, Flux, Argo CD, release pinning, and Helm guidance, see
[`docs/install.md`](../docs/install.md#gitops-install).
