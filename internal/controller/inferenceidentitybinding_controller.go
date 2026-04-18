/*
Copyright 2026 Kalin Daskalov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
)

const (
	defaultNameValue                  = "kleym"
	inferenceIdentityBindingFinalizer = "kleym.sonda.red/inferenceidentitybinding-finalizer"
	managedByLabelKey                 = "kleym.sonda.red/managed-by"
	managedByLabelValue               = defaultNameValue
	bindingNameLabelKey               = "kleym.sonda.red/binding-name"
	bindingNamespaceLabelKey          = "kleym.sonda.red/binding-namespace"
	defaultTrustDomain                = "kleym.sonda.red"

	conditionTypeReady          = "Ready"
	conditionTypeConflict       = "Conflict"
	conditionTypeInvalidRef     = "InvalidRef"
	conditionTypeUnsafeSelector = "UnsafeSelector"
	conditionTypeRenderFailure  = "RenderFailure"

	noIdentityCollisionMessage = "No identity collision detected"

	fieldIndexTargetRefName             = "spec.targetRef.name"
	fieldIndexEffectiveMode             = "spec.effectiveMode"
	fieldIndexContainerDiscriminatorKey = "spec.containerDiscriminatorKey"
	infraNotReadyRequeueAfter           = 30 * time.Second
	deleteVerificationRequeueAfter      = 2 * time.Second
	identityCollisionMessagePrefix      = "identity collision with bindings "
	identityCollisionMessageSuffix      = ": PerObjective bindings must not share the same pod selector and container discriminator"
	modeValuePerObjective               = string(kleymv1alpha1.InferenceIdentityBindingModePerObjective)
)

var (
	inferenceObjectiveGVKs = []schema.GroupVersionKind{
		{Group: "inference.networking.x-k8s.io", Version: "v1alpha2", Kind: "InferenceObjective"},
		{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferenceObjective"},
	}
	inferencePoolGVKs = []schema.GroupVersionKind{
		{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"},
		{Group: "inference.networking.x-k8s.io", Version: "v1alpha2", Kind: "InferencePool"},
	}
	clusterSPIFFEIDGVK = schema.GroupVersionKind{
		Group:   "spire.spiffe.io",
		Version: "v1alpha1",
		Kind:    "ClusterSPIFFEID",
	}
)

// InferenceIdentityBindingReconciler reconciles a InferenceIdentityBinding object
type InferenceIdentityBindingReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	Recorder               record.EventRecorder
	availableObjectiveGVKs []schema.GroupVersionKind
	availablePoolGVKs      []schema.GroupVersionKind
}

// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=inference.networking.k8s.io,resources=inferenceobjectives;inferencepools,verbs=get;list;watch
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferenceobjectives;inferencepools,verbs=get;list;watch
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InferenceIdentityBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).WithValues("inferenceIdentityBinding", req.NamespacedName)

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := r.Get(ctx, req.NamespacedName, binding); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !binding.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, binding)
	}

	if !controllerutil.ContainsFinalizer(binding, inferenceIdentityBindingFinalizer) {
		controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
		if err := r.Update(ctx, binding); err != nil {
			return ctrl.Result{}, err
		}
	}

	statusBase := binding.DeepCopy()
	initializeConditions(&binding.Status, binding.Generation)
	wasColliding := conditionIsTrue(statusBase.Status.Conditions, conditionTypeConflict)

	desiredState, err := r.computeDesiredState(ctx, binding, wasColliding)
	if err != nil {
		stateErr := &reconcileStateError{}
		if !errorsAsStateError(err, stateErr) {
			return ctrl.Result{}, err
		}
		if err := r.applyStateError(ctx, binding, stateErr); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
			return ctrl.Result{}, err
		}
		r.recordEventf(binding, corev1.EventTypeWarning, stateErr.reason, stateErr.message)
		if isInfrastructureNotReadyReason(stateErr.reason) {
			return ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}

	collisionResult, err := r.applyCollisionState(ctx, binding, desiredState.perObjectiveCollisionSet, wasColliding)
	if err != nil {
		return ctrl.Result{}, err
	}
	if collisionResult.currentHasCollision {
		applyCollisionStatus(&binding.Status, binding.Generation, true, collisionResult.currentMessage)
		if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
			return ctrl.Result{}, err
		}
		if collisionResult.currentDetected {
			r.recordEventf(binding, corev1.EventTypeWarning, "IdentityCollision", collisionResult.currentMessage)
		}
		if collisionResult.currentResolved {
			r.recordEventf(binding, corev1.EventTypeNormal, "IdentityCollisionResolved", "identity collision resolved")
		}
		logger.V(1).Info("skipping ClusterSPIFFEID reconciliation due to per-objective identity collision")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileClusterSPIFFEIDs(ctx, binding, desiredState.identities); err != nil {
		if meta.IsNoMatchError(err) {
			stateErr := newStateError(conditionTypeRenderFailure, "ClusterSPIFFEIDCRDMissing", "ClusterSPIFFEID CRD is not installed")
			if err := r.applyStateError(ctx, binding, stateErr); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
				return ctrl.Result{}, err
			}
			r.recordEventf(binding, corev1.EventTypeWarning, stateErr.reason, stateErr.message)
			return ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter}, nil
		}
		return ctrl.Result{}, err
	}

	applySuccessStatus(&binding.Status, binding.Generation, desiredState.identities)
	if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
		return ctrl.Result{}, err
	}

	if collisionResult.currentResolved {
		r.recordEventf(binding, corev1.EventTypeNormal, "IdentityCollisionResolved", "identity collision resolved")
	}

	primaryIdentity := desiredState.identities[0]
	r.recordEventf(binding, corev1.EventTypeNormal, "Reconciled", "reconciled ClusterSPIFFEID %q", primaryIdentity.Name)
	logger.V(1).Info("reconciled successfully", "clusterspiffeid", primaryIdentity.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceIdentityBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLogger := logf.Log.WithName("setup").WithName("inferenceidentitybinding")

	//nolint:staticcheck // We intentionally use the legacy recorder interface required by this reconciler.
	r.Recorder = mgr.GetEventRecorderFor("inferenceidentitybinding-controller")

	if err := r.setupFieldIndexes(mgr); err != nil {
		return err
	}

	availableObjectiveGVKs, err := filterAvailableGVKs(
		mgr.GetRESTMapper(),
		inferenceObjectiveGVKs,
		setupLogger.WithValues("resourceKind", "InferenceObjective"),
	)
	if err != nil {
		return err
	}

	availablePoolGVKs, err := filterAvailableGVKs(
		mgr.GetRESTMapper(),
		inferencePoolGVKs,
		setupLogger.WithValues("resourceKind", "InferencePool"),
	)
	if err != nil {
		return err
	}

	if len(availableObjectiveGVKs) == 0 && len(availablePoolGVKs) == 0 {
		return fmt.Errorf(
			"no supported GAIE GVKs are available: objective candidates=%v, pool candidates=%v",
			inferenceObjectiveGVKs,
			inferencePoolGVKs,
		)
	}

	r.availableObjectiveGVKs = append(
		make([]schema.GroupVersionKind, 0, len(availableObjectiveGVKs)),
		availableObjectiveGVKs...,
	)
	r.availablePoolGVKs = append(
		make([]schema.GroupVersionKind, 0, len(availablePoolGVKs)),
		availablePoolGVKs...,
	)

	watchPredicate := reconcileWatchPredicate()
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&kleymv1alpha1.InferenceIdentityBinding{}, builder.WithPredicates(watchPredicate)).
		Named("inferenceidentitybinding")

	for _, gvk := range availableObjectiveGVKs {
		objective := &unstructured.Unstructured{}
		objective.SetGroupVersionKind(gvk)
		controllerBuilder = controllerBuilder.Watches(
			objective,
			handler.EnqueueRequestsFromMapFunc(r.mapObjectiveToBindings),
			builder.WithPredicates(watchPredicate),
		)
	}

	for _, gvk := range availablePoolGVKs {
		pool := &unstructured.Unstructured{}
		pool.SetGroupVersionKind(gvk)
		controllerBuilder = controllerBuilder.Watches(
			pool,
			handler.EnqueueRequestsFromMapFunc(r.mapPoolToBindings),
			builder.WithPredicates(watchPredicate),
		)
	}

	return controllerBuilder.Complete(r)
}

func filterAvailableGVKs(
	mapper meta.RESTMapper,
	candidates []schema.GroupVersionKind,
	logger logr.Logger,
) ([]schema.GroupVersionKind, error) {
	available := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if _, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
			if meta.IsNoMatchError(err) {
				logger.Info("skipping unavailable GVK", "gvk", gvk.String())
				continue
			}
			return nil, fmt.Errorf("resolve REST mapping for %s: %w", gvk.String(), err)
		}
		available = append(available, gvk)
	}
	return available, nil
}
func (r *InferenceIdentityBindingReconciler) reconcileDelete(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(binding, inferenceIdentityBindingFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}

	remaining, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return ctrl.Result{}, err
		}
	} else if len(remaining) > 0 {
		return ctrl.Result{RequeueAfter: deleteVerificationRequeueAfter}, nil
	}

	controllerutil.RemoveFinalizer(binding, inferenceIdentityBindingFinalizer)
	if err := r.Update(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceIdentityBindingReconciler) computeDesiredState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	wasCurrentColliding bool,
) (desiredBindingState, error) {
	objective, err := r.resolveInferenceObjective(ctx, binding.Namespace, binding.Spec.TargetRef.Name)
	if err != nil {
		return desiredBindingState{}, err
	}

	poolRef, err := extractPoolRef(objective, binding.Namespace)
	if err != nil {
		return desiredBindingState{}, newStateError(conditionTypeInvalidRef, "InvalidPoolRef", err.Error())
	}

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return desiredBindingState{}, err
	}

	identity, err := r.renderIdentity(binding, objective, pool)
	if err != nil {
		return desiredBindingState{}, err
	}

	collisionSet, err := r.computePerObjectiveCollisionSet(ctx, binding, identity, wasCurrentColliding)
	if err != nil {
		return desiredBindingState{}, err
	}

	return desiredBindingState{
		identities:               []renderedIdentity{identity},
		perObjectiveCollisionSet: collisionSet,
	}, nil
}

func (r *InferenceIdentityBindingReconciler) applyStateError(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	stateErr *reconcileStateError,
) error {
	if shouldCleanupManagedClusterSPIFFEIDs(stateErr.conditionType) {
		if err := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
			return err
		}
	}

	applyFailureStatus(&binding.Status, binding.Generation, stateErr)
	return nil
}

func (r *InferenceIdentityBindingReconciler) applyCollisionState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	collisionSet perObjectiveCollisionSet,
	wasCurrentColliding bool,
) (collisionApplyResult, error) {
	currentBindingKey := namespacedBindingKey(binding.Namespace, binding.Name)
	result := collisionApplyResult{
		currentHasCollision: collisionSet.currentHasCollision,
		currentMessage:      collisionSet.currentMessage,
		currentDetected:     !wasCurrentColliding && collisionSet.currentHasCollision,
		currentResolved:     wasCurrentColliding && !collisionSet.currentHasCollision,
	}

	for i := range collisionSet.states {
		state := collisionSet.states[i]
		bindingKey := namespacedBindingKey(state.binding.Namespace, state.binding.Name)
		wasColliding := conditionIsTrue(state.binding.Status.Conditions, conditionTypeConflict)

		if state.hasCollision {
			if err := r.cleanupManagedClusterSPIFFEIDs(ctx, state.binding); err != nil {
				return collisionApplyResult{}, err
			}
		}

		if bindingKey == currentBindingKey {
			continue
		}

		if err := r.patchStatus(ctx, state.binding, func(status *kleymv1alpha1.InferenceIdentityBindingStatus) {
			initializeConditions(status, state.binding.Generation)
			applyCollisionStatus(status, state.binding.Generation, state.hasCollision, state.message)
		}); err != nil {
			return collisionApplyResult{}, err
		}

		if !wasColliding && state.hasCollision {
			r.recordEventf(state.binding, corev1.EventTypeWarning, "IdentityCollision", state.message)
		}
		if wasColliding && !state.hasCollision {
			r.recordEventf(state.binding, corev1.EventTypeNormal, "IdentityCollisionResolved", "identity collision resolved")
		}
	}

	if result.currentHasCollision {
		if err := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
			return collisionApplyResult{}, err
		}
	}

	return result, nil
}

func (r *InferenceIdentityBindingReconciler) recordEventf(
	object client.Object,
	eventType string,
	reason string,
	messageFormat string,
	args ...any,
) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(object, eventType, reason, messageFormat, args...)
}
