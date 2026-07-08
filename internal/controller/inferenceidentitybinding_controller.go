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
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/spirecm"
)

const (
	inferenceIdentityBindingFinalizer = "kleym.sonda.red/inferenceidentitybinding-finalizer"

	conditionTypeReady          = "Ready"
	conditionTypeInvalidRef     = "InvalidRef"
	conditionTypeUnsafeSelector = "UnsafeSelector"
	conditionTypeRenderFailure  = "RenderFailure"

	conditionReasonReconciled                = "Reconciled"
	conditionReasonResolved                  = "Resolved"
	conditionReasonInitializing              = "Initializing"
	conditionReasonInvalidPoolRef            = "InvalidPoolRef"
	conditionReasonClusterSPIFFEIDCRDMissing = "ClusterSPIFFEIDCRDMissing"

	fieldIndexPoolRefName          = "spec.poolRef.name"
	infraNotReadyRequeueAfter      = 30 * time.Second
	deleteVerificationRequeueAfter = 2 * time.Second

	logKeyBinding         = "binding"
	logKeyNamespace       = "namespace"
	logKeyName            = "name"
	logKeyPool            = "pool"
	logKeyPoolGVK         = "poolGVK"
	logKeyPoolGroup       = "poolGroup"
	logKeySpiffeID        = "spiffeID"
	logKeyClusterSPIFFEID = "clusterspiffeid"
	logKeySelectors       = "selectors"
	logKeyPodSelector     = "podSelector"
	logKeyCondition       = "condition"
	logKeyReason          = "reason"
	logKeyRequeueAfter    = "requeueAfter"
)

var (
	inferencePoolGVKs  = gaie.InferencePoolGVKs()
	clusterSPIFFEIDGVK = spirecm.ClusterSPIFFEIDGVK()
)

// InferenceIdentityBindingReconciler reconciles a InferenceIdentityBinding object
type InferenceIdentityBindingReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Config            OperatorConfig
	Recorder          events.EventRecorder
	MetricsRecorder   bindingOutcomeRecorder
	availablePoolGVKs []schema.GroupVersionKind
}

// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kleym.sonda.red,resources=inferenceidentitybindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=inference.networking.k8s.io,resources=inferencepools,verbs=get;list;watch
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile drives the InferenceIdentityBinding toward its desired state.
//
// The flow follows five mutually exclusive phases:
//
//  1. Fetch — get the binding; exit if not found.
//  2. Delete — if DeletionTimestamp is set, clean up ClusterSPIFFEIDs and remove the finalizer.
//  3. Finalizer — if not present, add and requeue (implicit via Update).
//  4. Compute + validate — resolve the pool reference and render identity.
//     Errors here are either:
//     - infrastructure-not-ready (CRD missing, transient) → requeue on a timer so recovery
//     does not depend on an unrelated watch event.
//     - permanent (invalid ref, unsafe selector) → set condition and stop.
//  5. Apply — reconcile ClusterSPIFFEID, patch status, emit events.
//
// Only one phase runs per invocation. Each phase either returns early or falls
// through to the next. Status is patched exactly once near the end of each path.
//
// See docs/design/reconciliation.md for the full flow diagram.
func (r *InferenceIdentityBindingReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	logger := logf.FromContext(ctx).WithValues(
		logKeyBinding, req.String(),
		logKeyNamespace, req.Namespace,
		logKeyName, req.Name,
	)
	ctx = logf.IntoContext(ctx, logger)

	logger.Info("starting InferenceIdentityBinding reconcile")
	defer func() {
		if reconcileErr != nil {
			logger.Error(reconcileErr, "finished InferenceIdentityBinding reconcile")
			return
		}
		logger.Info(
			"finished InferenceIdentityBinding reconcile",
			logKeyRequeueAfter, result.RequeueAfter,
		)
	}()

	// Phase 1: Fetch the binding.
	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := r.Get(ctx, req.NamespacedName, binding); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.V(1).Info("InferenceIdentityBinding not found")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger = logger.WithValues(
		logKeyPool, binding.Spec.PoolRef.Name,
	)
	ctx = logf.IntoContext(ctx, logger)

	// Phase 2: Handle deletion — clean up children, then remove finalizer.
	if !binding.DeletionTimestamp.IsZero() {
		logger.Info("handling deleted InferenceIdentityBinding")
		return r.reconcileDelete(ctx, binding)
	}

	// Phase 3: Ensure finalizer is present (requeues implicitly via Update).
	if !controllerutil.ContainsFinalizer(binding, inferenceIdentityBindingFinalizer) {
		logger.Info("adding InferenceIdentityBinding finalizer")
		controllerutil.AddFinalizer(binding, inferenceIdentityBindingFinalizer)
		if err := r.Update(ctx, binding); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Phase 4: Compute desired state — resolve pool → identity.
	statusBase := binding.DeepCopy()
	initializeConditions(&binding.Status, binding.Generation)
	applyOperatorConfig(&binding.Status, r.Config)

	desiredState, err := r.computeDesiredState(ctx, binding)
	if err != nil {
		// reconcileStateError carries condition type + reason + message so we can
		// set the right status condition from a single error return. Any other
		// error type is an unexpected failure and is returned to the controller
		// runtime for generic retry.
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
		r.recordTerminalOutcome(binding)
		logger.Info(
			"applied failure status",
			logKeyCondition, stateErr.conditionType,
			logKeyReason, stateErr.reason,
		)
		r.recordEventf(binding, corev1.EventTypeWarning, stateErr.reason, "%s", stateErr.message)
		// Infrastructure-not-ready (e.g. CRD missing) is transient: requeue on
		// a timer so recovery does not depend on an unrelated watch event.
		// Permanent errors (invalid ref, unsafe selector) stop here.
		if isInfrastructureNotReadyReason(stateErr.reason) {
			logger.Info(
				"requeueing after transient infrastructure readiness failure",
				logKeyReason, stateErr.reason,
				logKeyRequeueAfter, infraNotReadyRequeueAfter,
			)
			return ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}

	// Phase 5: Apply — reconcile ClusterSPIFFEID resources.
	managedStatuses, err := r.reconcileClusterSPIFFEIDs(ctx, binding, desiredState.identities)
	if err != nil {
		if meta.IsNoMatchError(err) {
			stateErr := newClusterSPIFFEIDCRDMissingStateError()
			if err := r.applyStateError(ctx, binding, stateErr); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
				return ctrl.Result{}, err
			}
			r.recordTerminalOutcome(binding)
			logger.Info(
				"applied failure status",
				logKeyCondition, stateErr.conditionType,
				logKeyReason, stateErr.reason,
			)
			r.recordEventf(binding, corev1.EventTypeWarning, stateErr.reason, "%s", stateErr.message)
			logger.Info(
				"requeueing after transient infrastructure readiness failure",
				logKeyReason, stateErr.reason,
				logKeyRequeueAfter, infraNotReadyRequeueAfter,
			)
			return ctrl.Result{RequeueAfter: infraNotReadyRequeueAfter}, nil
		}
		return ctrl.Result{}, err
	}

	applySuccessStatus(&binding.Status, binding.Generation, desiredState.identities, managedStatuses)
	if err := r.patchStatusFromBase(ctx, statusBase, binding); err != nil {
		return ctrl.Result{}, err
	}
	r.recordTerminalOutcome(binding)
	logger.Info(
		"applied success status",
		logKeyCondition, conditionTypeReady,
		logKeyReason, conditionReasonReconciled,
	)

	primaryIdentity := desiredState.identities[0]
	primaryClusterSPIFFEIDName := spirecm.DesiredClusterSPIFFEID(
		binding,
		primaryIdentity,
		r.Config.ClusterSPIFFEIDClassName,
	).GetName()
	r.recordEventf(binding, corev1.EventTypeNormal, conditionReasonReconciled, "reconciled ClusterSPIFFEID %q", primaryClusterSPIFFEIDName)
	logger.Info(
		"reconciled successfully",
		logKeyClusterSPIFFEID, primaryClusterSPIFFEIDName,
		logKeySpiffeID, primaryIdentity.SpiffeID,
	)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceIdentityBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLogger := logf.Log.WithName("setup").WithName("inferenceidentitybinding")

	if err := r.Config.Validate(); err != nil {
		return err
	}

	r.Recorder = mgr.GetEventRecorder("inferenceidentitybinding-controller")
	if r.MetricsRecorder == nil {
		r.MetricsRecorder = defaultBindingMetrics.outcomeRecorder
	}
	if err := defaultBindingMetrics.register(mgr.GetClient()); err != nil {
		return err
	}

	if err := r.setupFieldIndexes(mgr); err != nil {
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

	if len(availablePoolGVKs) == 0 {
		return fmt.Errorf(
			"no supported GAIE InferencePool GVKs are available: pool candidates=%v",
			inferencePoolGVKs,
		)
	}

	r.availablePoolGVKs = append(
		make([]schema.GroupVersionKind, 0, len(availablePoolGVKs)),
		availablePoolGVKs...,
	)

	watchPredicate := reconcileWatchPredicate()
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&kleymv1alpha1.InferenceIdentityBinding{}, builder.WithPredicates(watchPredicate)).
		Named("inferenceidentitybinding")

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
	logger := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(binding, inferenceIdentityBindingFinalizer) {
		logger.V(1).Info("deleted InferenceIdentityBinding has no finalizer")
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
		logger.Info(
			"waiting for managed ClusterSPIFFEIDs to disappear before removing finalizer",
			"remaining", len(remaining),
			logKeyRequeueAfter, deleteVerificationRequeueAfter,
		)
		return ctrl.Result{RequeueAfter: deleteVerificationRequeueAfter}, nil
	}

	logger.Info("removing InferenceIdentityBinding finalizer")
	controllerutil.RemoveFinalizer(binding, inferenceIdentityBindingFinalizer)
	if err := r.Update(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceIdentityBindingReconciler) computeDesiredState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (desiredBindingState, error) {
	logger := logf.FromContext(ctx)

	poolRef, err := gaie.BindingPoolRef(binding)
	if err != nil {
		return desiredBindingState{}, newStateError(conditionTypeInvalidRef, conditionReasonInvalidPoolRef, err.Error())
	}
	logger.Info(
		"resolved binding poolRef",
		logKeyPool, namespacedBindingKey(poolRef.Namespace, poolRef.Name),
		logKeyPoolGroup, poolRef.Group,
	)

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return desiredBindingState{}, err
	}
	logger.Info(
		"resolved target InferencePool",
		logKeyPool, namespacedBindingKey(pool.GetNamespace(), pool.GetName()),
		logKeyPoolGVK, pool.GroupVersionKind().String(),
	)

	identity, err := r.renderIdentity(binding, pool)
	if err != nil {
		return desiredBindingState{}, err
	}
	logger.Info(
		"rendered identity from inference intent",
		logKeySpiffeID, identity.SpiffeID,
		logKeyClusterSPIFFEID, spirecm.DesiredClusterSPIFFEID(
			binding,
			identity,
			r.Config.ClusterSPIFFEIDClassName,
		).GetName(),
		logKeySelectors, identity.Selectors,
		logKeyPodSelector, identity.PodSelector,
		logKeyPool, identity.PoolRef,
	)

	return desiredBindingState{
		identities: []renderedIdentity{identity},
	}, nil
}

func (r *InferenceIdentityBindingReconciler) applyStateError(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	stateErr *reconcileStateError,
) error {
	logger := logf.FromContext(ctx)
	if shouldCleanupManagedClusterSPIFFEIDs(stateErr.conditionType) {
		logger.Info(
			"cleaning up managed ClusterSPIFFEIDs after reconcile failure",
			logKeyCondition, stateErr.conditionType,
			logKeyReason, stateErr.reason,
		)
		if err := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); err != nil {
			return err
		}
	}

	applyFailureStatus(&binding.Status, binding.Generation, stateErr)
	return nil
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
	r.Recorder.Eventf(object, nil, eventType, reason, reason, messageFormat, args...)
}
