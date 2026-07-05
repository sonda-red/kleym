---
title: Kleym Diff Proof Cases
weight: 35
date: 2026-06-29
lastmod: 2026-06-29
summary: "Concrete cases for a future offline kleym diff command."
description: "Kleym diff cases for semantic inference identity changes across rendered SPIFFE IDs, selectors, and references."
---

Diff snippets below are excerpts; fixture files should contain complete
objects.

## Shared Base Case

The cases use this reference shape:

```yaml
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: reference-pool
  namespace: kleym-reference-inference
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: reference-model-server
      app.kubernetes.io/part-of: kleym-reference-inference
---
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferenceObjective
metadata:
  name: reference-objective
  namespace: kleym-reference-inference
spec:
  poolRef:
    name: reference-pool
---
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: binding
  namespace: kleym-reference-inference
spec:
  poolRef:
    name: reference-pool
  objectiveRef:
    name: reference-objective
  serviceAccountName: reference-inference
  mode: PerObjective
  containerName: model-server
```

Rendered identity fields:

```yaml
spec:
  spiffeIDTemplate: spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective
  podSelector:
    matchLabels:
      app.kubernetes.io/name: reference-model-server
      app.kubernetes.io/part-of: kleym-reference-inference
  workloadSelectorTemplates:
    - k8s:container-name:model-server
    - k8s:ns:kleym-reference-inference
    - k8s:pod-label:app.kubernetes.io/name:reference-model-server
    - k8s:pod-label:app.kubernetes.io/part-of:kleym-reference-inference
    - k8s:sa:reference-inference
  fallback: false
  hint: kleym-reference-inference/binding
```

## Cases

### Source Change With No Identity Delta

Input diff:

```diff
kind: InferenceObjective
metadata:
  name: reference-objective
  namespace: kleym-reference-inference
+  labels:
+    review.kleym.sonda.red/touched: "true"
```

Rendered impact: none.

Semantic result: `OK`, no identity delta.

### SPIFFE ID Changed

Input diff:

```diff
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferenceObjective
metadata:
-  name: reference-objective
+  name: reference-objective-v2
  namespace: kleym-reference-inference
spec:
  poolRef:
    name: reference-pool
---
apiVersion: kleym.sonda.red/v1alpha1
kind: InferenceIdentityBinding
metadata:
  name: binding
  namespace: kleym-reference-inference
spec:
  objectiveRef:
-    name: reference-objective
+    name: reference-objective-v2
```

Rendered impact:

```text
base SPIFFE ID: spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective
head SPIFFE ID: spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective-v2
```

Semantic result: `changed SPIFFE ID`.

### Service Account Changed

Input diff:

```diff
kind: InferenceIdentityBinding
spec:
-  serviceAccountName: reference-inference
+  serviceAccountName: reference-inference-v2
```

Rendered impact:

```text
unchanged SPIFFE ID: spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective
removed workload selector: k8s:sa:reference-inference
added workload selector: k8s:sa:reference-inference-v2
```

Semantic result: `changed workload selector set`.

### Pod Selector Widened

Input diff:

```diff
kind: InferencePool
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: reference-model-server
-      app.kubernetes.io/part-of: kleym-reference-inference
```

Rendered impact:

```text
removed pod selector label: app.kubernetes.io/part-of=kleym-reference-inference
removed derived workload selector: k8s:pod-label:app.kubernetes.io/part-of:kleym-reference-inference
```

Semantic result: `widened pod selector`.

### Invalid Objective Reference Introduced

Input diff:

```diff
kind: InferenceObjective
metadata:
  name: reference-objective
spec:
  poolRef:
-    name: reference-pool
+    name: other-pool
```

The binding still references `poolRef.name: reference-pool`.

Rendered impact: head cannot render identity output for the binding.

Semantic result: blocking finding `invalid reference introduced` with reason
`InvalidObjectiveRef`.

### Unsupported Pool Selector Introduced

Input diff:

```diff
kind: InferencePool
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: reference-model-server
      app.kubernetes.io/part-of: kleym-reference-inference
+    matchExpressions:
+      - key: app.kubernetes.io/part-of
+        operator: In
+        values:
+          - kleym-reference-inference
```

Rendered impact: head cannot render identity output for the binding. Current
GAIE selector derivation reports `pool spec.selector.matchExpressions are not
supported`.

Semantic result: blocking finding `unsupported pool selector introduced` with
reason `InvalidPoolSelector`.

## Separate Cases Needed

Collision reporting needs a peer-analysis case across multiple rendered
`PerObjective` identities. It is not covered by the one-binding cases above.
