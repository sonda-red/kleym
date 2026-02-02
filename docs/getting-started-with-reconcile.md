# Getting Started with Reconcile Logic

This guide walks you through implementing the reconcile loop for the InferenceTrustProfile controller, from first steps to a working MVP.

## Understanding the Reconcile Pattern

The reconcile function is called whenever:
- A resource is created, updated, or deleted
- A dependent resource (that you're watching) changes
- A requeue is requested

Your job: Make the actual cluster state match the desired state in the CR (Custom Resource).

## Step-by-Step Implementation Plan

### Phase 1: Basic Infrastructure (Start Here)

#### 1. Define Your Spec Fields

First, replace the placeholder `Foo` field in [api/v1alpha1/inferencetrustprofile_types.go](../api/v1alpha1/inferencetrustprofile_types.go) with actual fields. Based on the project goals, you need:

```go
type InferenceTrustProfileSpec struct {
    // TargetRef references the workload to attach identity to
    // This could be a Deployment, StatefulSet, or llm-d ModelService
    TargetRef WorkloadReference `json:"targetRef"`
    
    // SPIFFEConfig defines SPIFFE/SPIRE identity configuration
    // +optional
    SPIFFEConfig *SPIFFEConfig `json:"spiffeConfig,omitempty"`
    
    // MTLSRequired enforces mTLS for inference endpoint access
    // +kubebuilder:default=true
    MTLSRequired bool `json:"mtlsRequired"`
}

type WorkloadReference struct {
    // Kind of the target workload (Deployment, StatefulSet, ModelService)
    // +kubebuilder:validation:Enum=Deployment;StatefulSet;ModelService
    Kind string `json:"kind"`
    
    // Name of the target workload
    Name string `json:"name"`
}

type SPIFFEConfig struct {
    // SPIFFEIDTemplate defines the SPIFFE ID format
    // Example: spiffe://trust-domain/ns/{{.Namespace}}/sa/{{.ServiceAccount}}
    SPIFFEIDTemplate string `json:"spiffeIDTemplate"`
    
    // TrustDomain for SPIFFE identities
    // +kubebuilder:default="cluster.local"
    TrustDomain string `json:"trustDomain"`
}
```

After modifying types, **always run**:
```bash
make manifests generate
```

#### 2. Add Status Conditions

Update the Status struct to track reconciliation state:

```go
type InferenceTrustProfileStatus struct {
    // conditions represent the current state
    // +listType=map
    // +listMapKey=type
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // observedGeneration is the last generation reconciled
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // identityConfigured indicates if SPIFFE identity is attached
    // +optional
    IdentityConfigured bool `json:"identityConfigured,omitempty"`
    
    // targetWorkload references the actual workload being managed
    // +optional
    TargetWorkload *WorkloadReference `json:"targetWorkload,omitempty"`
}
```

Run `make manifests generate` again.

### Phase 2: Basic Reconcile Structure

#### 3. Implement Fetch-and-Validate Pattern

```go
func (r *InferenceTrustProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := logf.FromContext(ctx)
    
    // 1. Fetch the InferenceTrustProfile instance
    var profile terencesondaredv1alpha1.InferenceTrustProfile
    if err := r.Get(ctx, req.NamespacedName, &profile); err != nil {
        if apierrors.IsNotFound(err) {
            // Resource deleted, nothing to do
            log.Info("InferenceTrustProfile not found, likely deleted")
            return ctrl.Result{}, nil
        }
        log.Error(err, "Failed to get InferenceTrustProfile")
        return ctrl.Result{}, err
    }
    
    // 2. Handle deletion (if you add a finalizer later)
    // if !profile.DeletionTimestamp.IsZero() {
    //     return r.handleDeletion(ctx, &profile)
    // }
    
    // 3. Validate spec
    if err := r.validateSpec(&profile); err != nil {
        log.Error(err, "Invalid spec")
        return ctrl.Result{}, r.updateStatusCondition(ctx, &profile, "Ready", metav1.ConditionFalse, "ValidationFailed", err.Error())
    }
    
    // 4. Reconcile the actual resources
    if err := r.reconcileIdentity(ctx, &profile); err != nil {
        log.Error(err, "Failed to reconcile identity")
        return ctrl.Result{}, err
    }
    
    // 5. Update status
    profile.Status.ObservedGeneration = profile.Generation
    if err := r.Status().Update(ctx, &profile); err != nil {
        log.Error(err, "Failed to update status")
        return ctrl.Result{}, err
    }
    
    return ctrl.Result{}, nil
}
```

#### 4. Add Helper Functions

Create helper functions to keep your reconcile clean:

```go
func (r *InferenceTrustProfileReconciler) validateSpec(profile *terencesondaredv1alpha1.InferenceTrustProfile) error {
    if profile.Spec.TargetRef.Name == "" {
        return fmt.Errorf("targetRef.name is required")
    }
    if profile.Spec.TargetRef.Kind == "" {
        return fmt.Errorf("targetRef.kind is required")
    }
    return nil
}

func (r *InferenceTrustProfileReconciler) updateStatusCondition(
    ctx context.Context,
    profile *terencesondaredv1alpha1.InferenceTrustProfile,
    conditionType string,
    status metav1.ConditionStatus,
    reason, message string,
) error {
    meta.SetStatusCondition(&profile.Status.Conditions, metav1.Condition{
        Type:               conditionType,
        Status:             status,
        Reason:             reason,
        Message:            message,
        ObservedGeneration: profile.Generation,
    })
    return r.Status().Update(ctx, profile)
}
```

#### 5. Implement Core Business Logic

This is where you'll attach SPIFFE identity. Start simple:

```go
func (r *InferenceTrustProfileReconciler) reconcileIdentity(
    ctx context.Context,
    profile *terencesondaredv1alpha1.InferenceTrustProfile,
) error {
    log := logf.FromContext(ctx)
    
    // Step 1: Find the target workload
    targetWorkload, err := r.getTargetWorkload(ctx, profile)
    if err != nil {
        return fmt.Errorf("failed to get target workload: %w", err)
    }
    
    // Step 2: Check if workload has SPIFFE annotations
    needsUpdate := r.needsIdentityUpdate(targetWorkload, profile)
    
    if needsUpdate {
        // Step 3: Add SPIFFE annotations to pod template
        if err := r.attachIdentityAnnotations(ctx, targetWorkload, profile); err != nil {
            return fmt.Errorf("failed to attach identity: %w", err)
        }
        log.Info("Attached SPIFFE identity to workload", "workload", targetWorkload.GetName())
    }
    
    return nil
}
```

### Phase 3: Watch Dependent Resources

#### 6. Update SetupWithManager to Watch Workloads

```go
func (r *InferenceTrustProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&terencesondaredv1alpha1.InferenceTrustProfile{}).
        Owns(&appsv1.Deployment{}).  // Watch Deployments we manage
        Named("inferencetrustprofile").
        Complete(r)
}
```

You'll need to add:
```go
import (
    appsv1 "k8s.io/api/apps/v1"
)
```

And add RBAC markers:
```go
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
```

### Phase 4: Add Tests

#### 7. Write Unit Tests

In [internal/controller/inferencetrustprofile_controller_test.go](../internal/controller/inferencetrustprofile_controller_test.go):

```go
var _ = Describe("InferenceTrustProfile Controller", func() {
    Context("When reconciling a resource", func() {
        const resourceName = "test-profile"
        
        ctx := context.Background()
        
        typeNamespacedName := types.NamespacedName{
            Name:      resourceName,
            Namespace: "default",
        }
        
        BeforeEach(func() {
            By("creating the custom resource for the Kind InferenceTrustProfile")
            profile := &terencesondaredv1alpha1.InferenceTrustProfile{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      resourceName,
                    Namespace: "default",
                },
                Spec: terencesondaredv1alpha1.InferenceTrustProfileSpec{
                    TargetRef: terencesondaredv1alpha1.WorkloadReference{
                        Kind: "Deployment",
                        Name: "test-deployment",
                    },
                    MTLSRequired: true,
                },
            }
            Expect(k8sClient.Create(ctx, profile)).To(Succeed())
        })
        
        AfterEach(func() {
            resource := &terencesondaredv1alpha1.InferenceTrustProfile{}
            err := k8sClient.Get(ctx, typeNamespacedName, resource)
            Expect(err).NotTo(HaveOccurred())
            
            By("Cleanup the specific resource instance")
            Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
        })
        
        It("should successfully reconcile the resource", func() {
            By("Reconciling the created resource")
            controllerReconciler := &InferenceTrustProfileReconciler{
                Client: k8sClient,
                Scheme: k8sClient.Scheme(),
            }
            
            _, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
                NamespacedName: typeNamespacedName,
            })
            Expect(err).NotTo(HaveOccurred())
            
            // Add assertions about status, conditions, etc.
        })
    })
})
```

Run tests:
```bash
make test
```

## Quick Reference: Common Patterns

### Error Handling
```go
// Requeue with exponential backoff (automatic)
return ctrl.Result{}, err

// Requeue immediately
return ctrl.Result{Requeue: true}, nil

// Requeue after delay
return ctrl.Result{RequeueAfter: time.Minute * 5}, nil

// Done, no requeue needed
return ctrl.Result{}, nil
```

### Status Updates
Always use the status subresource client:
```go
if err := r.Status().Update(ctx, &profile); err != nil {
    return ctrl.Result{}, err
}
```

### Fetching Resources
```go
var deployment appsv1.Deployment
err := r.Get(ctx, types.NamespacedName{
    Name:      "my-deployment",
    Namespace: profile.Namespace,
}, &deployment)
```

### Listing Resources
```go
var deployments appsv1.DeploymentList
err := r.List(ctx, &deployments, 
    client.InNamespace(profile.Namespace),
    client.MatchingLabels{"app": "inference"},
)
```

## Development Workflow

1. **Edit types** in `api/v1alpha1/*_types.go`
2. **Run** `make manifests generate`
3. **Implement reconcile logic** in controller
4. **Add tests** in `*_test.go`
5. **Run tests** with `make test`
6. **Test locally** with `make install && make run`
7. **Deploy** with `make docker-build docker-push deploy`

## Next Steps

For your MVP, I recommend this order:

1. **Define proper Spec fields** (replace Foo) - targeting Deployments first
2. **Implement basic fetch and validate** logic
3. **Add SPIFFE annotation logic** (annotations on pod templates)
4. **Write unit tests**
5. **Test with local cluster** (`make install run`)
6. **Iterate** based on what you learn

## Useful Resources

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [controller-runtime godoc](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [SPIRE Kubernetes Quickstart](https://spiffe.io/docs/latest/try/getting-started-k8s/)
- [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)

## Common Gotchas

1. **Always run `make manifests generate`** after editing types
2. **Use status subresource** - never update status through regular Update()
3. **Handle NotFound errors** - resources can be deleted during reconciliation
4. **Make reconcile idempotent** - it can be called multiple times
5. **Don't block** - reconcile should be fast; use background jobs for slow operations
6. **Check generation** - use `ObservedGeneration` to detect spec changes

## Questions to Ask Yourself

- What resources do I need to create/update?
- What resources do I need to watch?
- What RBAC permissions do I need? (add `+kubebuilder:rbac` markers)
- What should happen when my CR is deleted?
- How do I know if reconciliation succeeded?
- What conditions should I set in status?
