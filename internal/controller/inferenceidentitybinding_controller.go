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
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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

	objective, err := r.resolveInferenceObjective(ctx, binding.Namespace, binding.Spec.TargetRef.Name)
	if err != nil {
		return r.handleReconcileStateError(ctx, binding, err)
	}

	poolRef, err := extractPoolRef(objective, binding.Namespace)
	if err != nil {
		return r.handleReconcileStateError(ctx, binding, newStateError(conditionTypeInvalidRef, "InvalidPoolRef", err.Error()))
	}

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return r.handleReconcileStateError(ctx, binding, err)
	}

	identity, err := r.renderIdentity(binding, objective, pool)
	if err != nil {
		return r.handleReconcileStateError(ctx, binding, err)
	}

	hasCollision, err := r.reconcilePerObjectiveCollisionState(ctx, binding, identity)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hasCollision {
		logger.V(1).Info("skipping ClusterSPIFFEID reconciliation due to per-objective identity collision")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileClusterSPIFFEIDs(ctx, binding, []renderedIdentity{identity}); err != nil {
		if meta.IsNoMatchError(err) {
			return r.handleReconcileStateError(
				ctx,
				binding,
				newStateError(conditionTypeRenderFailure, "ClusterSPIFFEIDCRDMissing", "ClusterSPIFFEID CRD is not installed"),
			)
		}
		return ctrl.Result{}, err
	}

	if err := r.updateSuccessStatus(ctx, binding, []renderedIdentity{identity}); err != nil {
		return ctrl.Result{}, err
	}

	r.recordEventf(binding, corev1.EventTypeNormal, "Reconciled", "reconciled ClusterSPIFFEID %q", identity.Name)
	logger.V(1).Info("reconciled successfully", "clusterspiffeid", identity.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceIdentityBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:staticcheck // We intentionally use the legacy recorder interface required by this reconciler.
	r.Recorder = mgr.GetEventRecorderFor("inferenceidentitybinding-controller")

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&kleymv1alpha1.InferenceIdentityBinding{}).
		Named("inferenceidentitybinding")

	for _, gvk := range inferenceObjectiveGVKs {
		objective := &unstructured.Unstructured{}
		objective.SetGroupVersionKind(gvk)
		controllerBuilder = controllerBuilder.Watches(
			objective,
			handler.EnqueueRequestsFromMapFunc(r.mapObjectiveToBindings),
		)
	}

	for _, gvk := range inferencePoolGVKs {
		pool := &unstructured.Unstructured{}
		pool.SetGroupVersionKind(gvk)
		controllerBuilder = controllerBuilder.Watches(
			pool,
			handler.EnqueueRequestsFromMapFunc(r.mapPoolToBindings),
		)
	}

	return controllerBuilder.Complete(r)
}

type reconcileStateError struct {
	conditionType string
	reason        string
	message       string
}

func (e *reconcileStateError) Error() string {
	return e.message
}

func newStateError(conditionType, reason, message string) *reconcileStateError {
	return &reconcileStateError{
		conditionType: conditionType,
		reason:        reason,
		message:       message,
	}
}

type inferencePoolRef struct {
	Name      string
	Group     string
	Namespace string
}

type renderedIdentity struct {
	Name         string
	Mode         kleymv1alpha1.InferenceIdentityBindingMode
	SpiffeID     string
	Selectors    []string
	PodSelector  map[string]any
	ObjectiveRef string
	PoolRef      string
}

type renderTemplateData struct {
	Namespace                   string
	BindingName                 string
	ObjectiveName               string
	PoolName                    string
	Mode                        string
	ContainerDiscriminatorType  string
	ContainerDiscriminatorValue string
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

	controllerutil.RemoveFinalizer(binding, inferenceIdentityBindingFinalizer)
	if err := r.Update(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceIdentityBindingReconciler) resolveInferenceObjective(
	ctx context.Context,
	namespace string,
	name string,
) (*unstructured.Unstructured, error) {
	objective, _, crdMissing, err := r.resolveByCandidates(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: name},
		inferenceObjectiveGVKs,
	)
	if err != nil {
		return nil, err
	}
	if objective != nil {
		return objective, nil
	}
	if crdMissing {
		return nil, newStateError(
			conditionTypeInvalidRef,
			"InferenceObjectiveCRDMissing",
			"InferenceObjective CRD is not installed",
		)
	}
	return nil, newStateError(
		conditionTypeInvalidRef,
		"TargetObjectiveNotFound",
		fmt.Sprintf("targetRef %q was not found", name),
	)
}

func (r *InferenceIdentityBindingReconciler) resolveInferencePool(
	ctx context.Context,
	poolRef inferencePoolRef,
) (*unstructured.Unstructured, error) {
	poolCandidates := candidatePoolGVKs(poolRef.Group)
	pool, _, crdMissing, err := r.resolveByCandidates(
		ctx,
		types.NamespacedName{Namespace: poolRef.Namespace, Name: poolRef.Name},
		poolCandidates,
	)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		return pool, nil
	}
	if crdMissing {
		return nil, newStateError(
			conditionTypeInvalidRef,
			"InferencePoolCRDMissing",
			"InferencePool CRD is not installed",
		)
	}
	return nil, newStateError(
		conditionTypeInvalidRef,
		"TargetPoolNotFound",
		fmt.Sprintf("poolRef %q was not found", poolRef.Name),
	)
}

func (r *InferenceIdentityBindingReconciler) resolveByCandidates(
	ctx context.Context,
	key types.NamespacedName,
	candidates []schema.GroupVersionKind,
) (*unstructured.Unstructured, schema.GroupVersionKind, bool, error) {
	crdMissing := false

	for _, gvk := range candidates {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		err := r.Get(ctx, key, obj)
		switch {
		case err == nil:
			return obj, gvk, crdMissing, nil
		case apierrors.IsNotFound(err):
			continue
		case meta.IsNoMatchError(err):
			crdMissing = true
			continue
		default:
			return nil, schema.GroupVersionKind{}, crdMissing, err
		}
	}

	return nil, schema.GroupVersionKind{}, crdMissing, nil
}

func extractPoolRef(objective *unstructured.Unstructured, defaultNamespace string) (inferencePoolRef, error) {
	poolRefMap, found, err := unstructured.NestedMap(objective.Object, "spec", "poolRef")
	if err != nil {
		return inferencePoolRef{}, fmt.Errorf("failed to decode objective spec.poolRef: %w", err)
	}
	if !found {
		return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef is required")
	}

	name, ok := poolRefMap["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.name is required")
	}

	group := ""
	if rawGroup, exists := poolRefMap["group"]; exists {
		groupValue, groupOK := rawGroup.(string)
		if !groupOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.group must be a string")
		}
		group = strings.TrimSpace(groupValue)
	}

	namespace := defaultNamespace
	if rawNamespace, exists := poolRefMap["namespace"]; exists {
		namespaceValue, namespaceOK := rawNamespace.(string)
		if !namespaceOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.namespace must be a string")
		}
		if namespaceValue != "" && namespaceValue != defaultNamespace {
			return inferencePoolRef{}, fmt.Errorf("cross-namespace poolRef is not allowed")
		}
		if namespaceValue != "" {
			namespace = namespaceValue
		}
	}

	if rawKind, exists := poolRefMap["kind"]; exists {
		kindValue, kindOK := rawKind.(string)
		if !kindOK {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be a string")
		}
		if kindValue != "" && kindValue != "InferencePool" {
			return inferencePoolRef{}, fmt.Errorf("objective spec.poolRef.kind must be InferencePool")
		}
	}

	return inferencePoolRef{
		Name:      strings.TrimSpace(name),
		Group:     group,
		Namespace: namespace,
	}, nil
}

func candidatePoolGVKs(group string) []schema.GroupVersionKind {
	if group == "" {
		return inferencePoolGVKs
	}

	filtered := make([]schema.GroupVersionKind, 0, len(inferencePoolGVKs))
	for _, gvk := range inferencePoolGVKs {
		if gvk.Group == group {
			filtered = append(filtered, gvk)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}

	return []schema.GroupVersionKind{
		{Group: group, Version: "v1", Kind: "InferencePool"},
		{Group: group, Version: "v1alpha2", Kind: "InferencePool"},
	}
}

func (r *InferenceIdentityBindingReconciler) renderIdentity(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
) (renderedIdentity, error) {
	mode := effectiveMode(binding.Spec.Mode)
	if mode != kleymv1alpha1.InferenceIdentityBindingModePoolOnly &&
		mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"UnsupportedMode",
			fmt.Sprintf("unsupported mode %q", mode),
		)
	}

	podSelector, poolDerivedSelectors, err := deriveSelectorsFromPool(pool)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeUnsafeSelector,
			"InvalidPoolSelector",
			err.Error(),
		)
	}

	templateData := renderTemplateData{
		Namespace:     binding.Namespace,
		BindingName:   binding.Name,
		ObjectiveName: objective.GetName(),
		PoolName:      pool.GetName(),
		Mode:          string(mode),
	}
	if binding.Spec.ContainerDiscriminator != nil {
		templateData.ContainerDiscriminatorType = string(binding.Spec.ContainerDiscriminator.Type)
		templateData.ContainerDiscriminatorValue = binding.Spec.ContainerDiscriminator.Value
	}

	renderedSelectors, err := renderSelectorTemplates(binding.Spec.WorkloadSelectorTemplates, templateData)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"SelectorTemplateRenderFailed",
			err.Error(),
		)
	}

	selectors := append(renderedSelectors, poolDerivedSelectors...)
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		if binding.Spec.ContainerDiscriminator == nil {
			return renderedIdentity{}, newStateError(
				conditionTypeRenderFailure,
				"MissingContainerDiscriminator",
				"containerDiscriminator is required when mode is PerObjective",
			)
		}
		containerSelector, selectorErr := selectorForContainerDiscriminator(binding.Spec.ContainerDiscriminator)
		if selectorErr != nil {
			return renderedIdentity{}, newStateError(
				conditionTypeRenderFailure,
				"InvalidContainerDiscriminator",
				selectorErr.Error(),
			)
		}
		selectors = append(selectors, containerSelector)
	}

	selectors = uniqueAndSorted(selectors)
	if err := validateSafetySelectors(binding.Namespace, selectors); err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeUnsafeSelector,
			"UnsafeSelector",
			err.Error(),
		)
	}

	spiffeID, err := renderSPIFFEID(binding.Spec.SpiffeIDTemplate, mode, templateData)
	if err != nil {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"SPIFFEIDRenderFailed",
			err.Error(),
		)
	}
	if !strings.HasPrefix(spiffeID, "spiffe://") {
		return renderedIdentity{}, newStateError(
			conditionTypeRenderFailure,
			"InvalidSPIFFEID",
			fmt.Sprintf("computed SPIFFE ID %q must start with spiffe://", spiffeID),
		)
	}

	return renderedIdentity{
		Name:         buildClusterSPIFFEIDName(binding.Namespace, binding.Name, mode, spiffeID),
		Mode:         mode,
		SpiffeID:     spiffeID,
		Selectors:    selectors,
		PodSelector:  podSelector,
		ObjectiveRef: objective.GetName(),
		PoolRef:      pool.GetName(),
	}, nil
}

type perObjectiveCollisionCandidate struct {
	binding *kleymv1alpha1.InferenceIdentityBinding
	key     string
}

func (r *InferenceIdentityBindingReconciler) reconcilePerObjectiveCollisionState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
) (bool, error) {
	if identity.Mode != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		_, resolved, err := r.updateCollisionCondition(ctx, binding, false, noIdentityCollisionMessage)
		if err != nil {
			return false, err
		}
		if resolved {
			r.recordEventf(binding, corev1.EventTypeNormal, "IdentityCollisionResolved", "identity collision resolved")
		}
	}

	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(binding.Namespace)); err != nil {
		return false, err
	}

	candidates := make([]perObjectiveCollisionCandidate, 0, len(bindingList.Items))
	currentBindingKey := namespacedBindingKey(binding.Namespace, binding.Name)
	for i := range bindingList.Items {
		candidateBinding := bindingList.Items[i].DeepCopy()
		if !candidateBinding.DeletionTimestamp.IsZero() {
			continue
		}
		if effectiveMode(candidateBinding.Spec.Mode) != kleymv1alpha1.InferenceIdentityBindingModePerObjective {
			continue
		}

		candidateKey := namespacedBindingKey(candidateBinding.Namespace, candidateBinding.Name)
		var candidateIdentity renderedIdentity
		if candidateKey == currentBindingKey {
			candidateIdentity = identity
		} else {
			resolvedIdentity, resolveErr := r.renderIdentityForBinding(ctx, candidateBinding)
			if resolveErr != nil {
				continue
			}
			candidateIdentity = resolvedIdentity
		}

		fingerprint, fingerprintErr := perObjectiveCollisionFingerprint(candidateIdentity, candidateBinding.Spec.ContainerDiscriminator)
		if fingerprintErr != nil {
			continue
		}

		candidates = append(candidates, perObjectiveCollisionCandidate{
			binding: candidateBinding,
			key:     fingerprint,
		})
	}

	groups := make(map[string][]int, len(candidates))
	for i, candidate := range candidates {
		groups[candidate.key] = append(groups[candidate.key], i)
	}

	collidingByBinding := make(map[string]bool, len(candidates))
	messageByBinding := make(map[string]string, len(candidates))
	for _, indexes := range groups {
		if len(indexes) < 2 {
			for _, idx := range indexes {
				candidate := candidates[idx]
				messageByBinding[namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)] = noIdentityCollisionMessage
			}
			continue
		}

		memberNames := make([]string, 0, len(indexes))
		for _, idx := range indexes {
			memberNames = append(memberNames, candidates[idx].binding.Name)
		}
		sort.Strings(memberNames)

		for _, idx := range indexes {
			candidate := candidates[idx]
			bindingKey := namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)
			collidingByBinding[bindingKey] = true
			messageByBinding[bindingKey] = identityCollisionMessage(candidate.binding.Name, memberNames)
		}
	}

	for i := range candidates {
		candidate := &candidates[i]
		bindingKey := namespacedBindingKey(candidate.binding.Namespace, candidate.binding.Name)
		colliding := collidingByBinding[bindingKey]
		message := messageByBinding[bindingKey]
		if message == "" {
			message = noIdentityCollisionMessage
		}

		detected, resolved, err := r.updateCollisionCondition(ctx, candidate.binding, colliding, message)
		if err != nil {
			return false, err
		}

		if colliding {
			if err := r.cleanupManagedClusterSPIFFEIDs(ctx, candidate.binding); err != nil {
				return false, err
			}
			if detected {
				r.recordEventf(candidate.binding, corev1.EventTypeWarning, "IdentityCollision", message)
			}
			continue
		}
		if resolved {
			r.recordEventf(candidate.binding, corev1.EventTypeNormal, "IdentityCollisionResolved", "identity collision resolved")
		}
	}

	return collidingByBinding[currentBindingKey], nil
}

func (r *InferenceIdentityBindingReconciler) renderIdentityForBinding(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) (renderedIdentity, error) {
	objective, err := r.resolveInferenceObjective(ctx, binding.Namespace, binding.Spec.TargetRef.Name)
	if err != nil {
		return renderedIdentity{}, err
	}

	poolRef, err := extractPoolRef(objective, binding.Namespace)
	if err != nil {
		return renderedIdentity{}, err
	}

	pool, err := r.resolveInferencePool(ctx, poolRef)
	if err != nil {
		return renderedIdentity{}, err
	}

	return r.renderIdentity(binding, objective, pool)
}

func (r *InferenceIdentityBindingReconciler) updateCollisionCondition(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	hasCollision bool,
	message string,
) (detected bool, resolved bool, err error) {
	previousCondition := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeConflict)
	wasColliding := previousCondition != nil && previousCondition.Status == metav1.ConditionTrue

	if err := r.patchStatus(ctx, binding, func(status *kleymv1alpha1.InferenceIdentityBindingStatus) {
		if hasCollision {
			status.ComputedSpiffeIDs = nil
			status.RenderedSelectors = nil
			setCondition(status, binding.Generation, conditionTypeReady, metav1.ConditionFalse, "IdentityCollision", message)
			setCondition(status, binding.Generation, conditionTypeConflict, metav1.ConditionTrue, "IdentityCollision", message)
			setCondition(status, binding.Generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
			setCondition(status, binding.Generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
			setCondition(status, binding.Generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
			return
		}

		setCondition(status, binding.Generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", noIdentityCollisionMessage)
	}); err != nil {
		return false, false, err
	}

	return !wasColliding && hasCollision, wasColliding && !hasCollision, nil
}

func perObjectiveCollisionFingerprint(
	identity renderedIdentity,
	discriminator *kleymv1alpha1.ContainerDiscriminator,
) (string, error) {
	if discriminator == nil {
		return "", fmt.Errorf("containerDiscriminator is required for per-objective collision detection")
	}

	containerValue := strings.TrimSpace(discriminator.Value)
	if containerValue == "" {
		return "", fmt.Errorf("containerDiscriminator.value must not be empty")
	}

	podSelectorFingerprint, err := normalizedPodSelectorFingerprint(identity.PodSelector)
	if err != nil {
		return "", err
	}

	selectorFingerprint, err := normalizedSelectorFingerprint(identity.Selectors)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s|%s|%s|%s", podSelectorFingerprint, selectorFingerprint, discriminator.Type, containerValue), nil
}

func normalizedPodSelectorFingerprint(selector map[string]any) (string, error) {
	if len(selector) == 0 {
		return "", fmt.Errorf("pod selector must be present for collision detection")
	}

	serialized, err := json.Marshal(selector)
	if err != nil {
		return "", fmt.Errorf("failed to encode pod selector fingerprint: %w", err)
	}

	return string(serialized), nil
}

func normalizedSelectorFingerprint(selectors []string) (string, error) {
	if len(selectors) == 0 {
		return "", fmt.Errorf("selectors must be present for collision detection")
	}

	serialized, err := json.Marshal(selectors)
	if err != nil {
		return "", fmt.Errorf("failed to encode selector fingerprint: %w", err)
	}

	return string(serialized), nil
}

func identityCollisionMessage(bindingName string, collidingBindings []string) string {
	peers := make([]string, 0, len(collidingBindings))
	for _, name := range collidingBindings {
		if name == bindingName {
			continue
		}
		peers = append(peers, name)
	}
	sort.Strings(peers)
	return fmt.Sprintf(
		"identity collision with bindings %s: PerObjective bindings must not share the same pod selector and container discriminator",
		strings.Join(peers, ", "),
	)
}

func namespacedBindingKey(namespace, name string) string {
	return types.NamespacedName{Namespace: namespace, Name: name}.String()
}

func effectiveMode(mode kleymv1alpha1.InferenceIdentityBindingMode) kleymv1alpha1.InferenceIdentityBindingMode {
	if mode == "" {
		return kleymv1alpha1.InferenceIdentityBindingModePerObjective
	}
	return mode
}

func deriveSelectorsFromPool(pool *unstructured.Unstructured) (map[string]any, []string, error) {
	selectorMap, found, err := unstructured.NestedMap(pool.Object, "spec", "selector")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode pool spec.selector: %w", err)
	}
	if !found || len(selectorMap) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector must be set")
	}

	var matchLabels map[string]any
	if rawMatchLabels, hasMatchLabels := selectorMap["matchLabels"]; hasMatchLabels {
		typedMatchLabels, ok := rawMatchLabels.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must be an object")
		}
		matchLabels = typedMatchLabels
	} else {
		isFlatSelector := true
		for _, value := range selectorMap {
			if _, ok := value.(string); !ok {
				isFlatSelector = false
				break
			}
		}
		if !isFlatSelector {
			return nil, nil, fmt.Errorf("pool selector must use matchLabels for deterministic rendering")
		}
		matchLabels = selectorMap
		selectorMap = map[string]any{"matchLabels": matchLabels}
	}

	if rawExpressions, hasExpressions := selectorMap["matchExpressions"]; hasExpressions {
		expressions, ok := rawExpressions.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions must be an array")
		}
		if len(expressions) > 0 {
			return nil, nil, fmt.Errorf("pool spec.selector.matchExpressions are not supported")
		}
	}

	if len(matchLabels) == 0 {
		return nil, nil, fmt.Errorf("pool spec.selector.matchLabels must not be empty")
	}

	derivedSelectors := make([]string, 0, len(matchLabels))
	for key, value := range matchLabels {
		valueText := strings.TrimSpace(fmt.Sprintf("%v", value))
		if key == "" || valueText == "" {
			return nil, nil, fmt.Errorf("pool selector labels must contain non-empty keys and values")
		}
		derivedSelectors = append(derivedSelectors, fmt.Sprintf("k8s:pod-label:%s:%s", key, valueText))
	}

	return selectorMap, derivedSelectors, nil
}

func renderSelectorTemplates(templates []string, data renderTemplateData) ([]string, error) {
	rendered := make([]string, 0, len(templates))
	for i, selectorTemplate := range templates {
		value, err := renderTemplate("selector", fmt.Sprintf("selector-%d", i), selectorTemplate, data)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, value)
	}
	return rendered, nil
}

func selectorForContainerDiscriminator(discriminator *kleymv1alpha1.ContainerDiscriminator) (string, error) {
	value := strings.TrimSpace(discriminator.Value)
	if value == "" {
		return "", fmt.Errorf("containerDiscriminator.value must not be empty")
	}

	switch discriminator.Type {
	case kleymv1alpha1.ContainerDiscriminatorTypeName:
		return "k8s:container-name:" + value, nil
	case kleymv1alpha1.ContainerDiscriminatorTypeImage:
		return "k8s:container-image:" + value, nil
	default:
		return "", fmt.Errorf("unsupported containerDiscriminator.type %q", discriminator.Type)
	}
}

func validateSafetySelectors(namespace string, selectors []string) error {
	hasNamespaceSelector := false
	hasServiceAccountSelector := false

	for _, selector := range selectors {
		switch {
		case strings.HasPrefix(selector, "k8s:ns:"):
			hasNamespaceSelector = true
			ns := strings.TrimPrefix(selector, "k8s:ns:")
			if ns != namespace {
				return fmt.Errorf("selector %q escapes binding namespace %q", selector, namespace)
			}
		case strings.HasPrefix(selector, "k8s:sa:"):
			serviceAccount := strings.TrimPrefix(selector, "k8s:sa:")
			if strings.TrimSpace(serviceAccount) == "" {
				return fmt.Errorf("service account selector must not be empty")
			}
			hasServiceAccountSelector = true
		}
	}

	if !hasNamespaceSelector {
		return fmt.Errorf("selectors must include k8s:ns:%s", namespace)
	}
	if !hasServiceAccountSelector {
		return fmt.Errorf("selectors must include a k8s:sa:<service-account> selector")
	}

	return nil
}

func renderSPIFFEID(
	customTemplate *string,
	mode kleymv1alpha1.InferenceIdentityBindingMode,
	data renderTemplateData,
) (string, error) {
	if customTemplate == nil {
		switch mode {
		case kleymv1alpha1.InferenceIdentityBindingModePoolOnly:
			return fmt.Sprintf("spiffe://%s/ns/%s/pool/%s", defaultTrustDomain, data.Namespace, data.PoolName), nil
		case kleymv1alpha1.InferenceIdentityBindingModePerObjective:
			return fmt.Sprintf("spiffe://%s/ns/%s/objective/%s", defaultTrustDomain, data.Namespace, data.ObjectiveName), nil
		default:
			return "", fmt.Errorf("unsupported mode %q", mode)
		}
	}

	return renderTemplate("spiffeID", "spiffeIDTemplate", *customTemplate, data)
}

func renderTemplate(kind, name, source string, data renderTemplateData) (string, error) {
	parsed, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("%s template parse failed: %w", kind, err)
	}

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("%s template render failed: %w", kind, err)
	}

	value := strings.TrimSpace(rendered.String())
	if value == "" {
		return "", fmt.Errorf("%s template rendered to an empty value", kind)
	}

	return value, nil
}

func buildClusterSPIFFEIDName(
	namespace string,
	bindingName string,
	mode kleymv1alpha1.InferenceIdentityBindingMode,
	spiffeID string,
) string {
	modeText := "pool"
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective {
		modeText = "objective"
	}

	hashSum := sha1.Sum([]byte(spiffeID))
	hashSuffix := hex.EncodeToString(hashSum[:4])
	base := sanitizeDNSLabel(fmt.Sprintf("%s-%s-%s", defaultNameValue, namespace, bindingName))

	maxBaseLen := 63 - len(modeText) - len(hashSuffix) - 2
	if maxBaseLen < 1 {
		maxBaseLen = 1
	}
	if len(base) > maxBaseLen {
		base = strings.Trim(base[:maxBaseLen], "-")
		if base == "" {
			base = defaultNameValue
		}
	}

	return fmt.Sprintf("%s-%s-%s", base, modeText, hashSuffix)
}

func sanitizeDNSLabel(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return defaultNameValue
	}

	var builder strings.Builder
	lastHyphen := false
	for _, character := range lower {
		isAlphaNum := (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
		if isAlphaNum {
			builder.WriteRune(character)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			builder.WriteRune('-')
			lastHyphen = true
		}
	}

	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return defaultNameValue
	}

	return sanitized
}

func uniqueAndSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}

	unique := make([]string, 0, len(set))
	for value := range set {
		unique = append(unique, value)
	}
	sort.Strings(unique)

	return unique
}

func (r *InferenceIdentityBindingReconciler) reconcileClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identities []renderedIdentity,
) error {
	existing, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		return err
	}

	existingByName := make(map[string]*unstructured.Unstructured, len(existing))
	for _, item := range existing {
		existingByName[item.GetName()] = item
	}

	desiredNames := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		desired := desiredClusterSPIFFEID(binding, identity)
		desiredNames[identity.Name] = struct{}{}

		current, exists := existingByName[identity.Name]
		if !exists {
			if err := r.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			continue
		}

		if !clusterSPIFFEIDInSync(current, desired) {
			mergeDesiredClusterSPIFFEID(current, desired)
			if err := r.Update(ctx, current); err != nil {
				return err
			}
		}
	}

	for name, object := range existingByName {
		if _, keep := desiredNames[name]; keep {
			continue
		}
		if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func desiredClusterSPIFFEID(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identity renderedIdentity,
) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(clusterSPIFFEIDGVK)
	object.SetName(identity.Name)
	object.SetLabels(map[string]string{
		managedByLabelKey:        managedByLabelValue,
		bindingNameLabelKey:      binding.Name,
		bindingNamespaceLabelKey: binding.Namespace,
	})

	selectorTemplates := make([]any, 0, len(identity.Selectors))
	for _, selector := range identity.Selectors {
		selectorTemplates = append(selectorTemplates, selector)
	}

	object.Object["spec"] = map[string]any{
		"spiffeIDTemplate":          identity.SpiffeID,
		"podSelector":               identity.PodSelector,
		"workloadSelectorTemplates": selectorTemplates,
	}

	return object
}

func clusterSPIFFEIDInSync(current *unstructured.Unstructured, desired *unstructured.Unstructured) bool {
	currentSpec, _, currentErr := unstructured.NestedMap(current.Object, "spec")
	desiredSpec, _, desiredErr := unstructured.NestedMap(desired.Object, "spec")
	if currentErr != nil || desiredErr != nil {
		return false
	}

	if !reflect.DeepEqual(currentSpec, desiredSpec) {
		return false
	}

	currentLabels := current.GetLabels()
	for key, value := range desired.GetLabels() {
		if currentLabels[key] != value {
			return false
		}
	}

	return true
}

func mergeDesiredClusterSPIFFEID(current *unstructured.Unstructured, desired *unstructured.Unstructured) {
	currentSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	if current.Object == nil {
		current.Object = map[string]any{}
	}
	current.Object["spec"] = currentSpec

	labels := current.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range desired.GetLabels() {
		labels[key] = value
	}
	current.SetLabels(labels)
}

func (r *InferenceIdentityBindingReconciler) listManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]*unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(clusterSPIFFEIDGVK.GroupVersion().WithKind(clusterSPIFFEIDGVK.Kind + "List"))

	if err := r.List(
		ctx,
		list,
		client.MatchingLabels(map[string]string{
			managedByLabelKey:        managedByLabelValue,
			bindingNameLabelKey:      binding.Name,
			bindingNamespaceLabelKey: binding.Namespace,
		}),
	); err != nil {
		return nil, err
	}

	items := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, list.Items[i].DeepCopy())
	}

	return items, nil
}

func (r *InferenceIdentityBindingReconciler) cleanupManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) error {
	objects, err := r.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	for _, object := range objects {
		if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *InferenceIdentityBindingReconciler) handleReconcileStateError(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	err error,
) (ctrl.Result, error) {
	stateErr := &reconcileStateError{}
	if !errorsAsStateError(err, stateErr) {
		return ctrl.Result{}, err
	}

	if stateErr.conditionType == conditionTypeInvalidRef ||
		stateErr.conditionType == conditionTypeUnsafeSelector ||
		stateErr.conditionType == conditionTypeRenderFailure ||
		stateErr.conditionType == conditionTypeConflict {
		if cleanupErr := r.cleanupManagedClusterSPIFFEIDs(ctx, binding); cleanupErr != nil {
			return ctrl.Result{}, cleanupErr
		}
	}

	if statusErr := r.updateFailureStatus(ctx, binding, stateErr); statusErr != nil {
		return ctrl.Result{}, statusErr
	}
	r.recordEventf(binding, corev1.EventTypeWarning, stateErr.reason, stateErr.message)
	return ctrl.Result{}, nil
}

func errorsAsStateError(err error, target *reconcileStateError) bool {
	stateErr, ok := err.(*reconcileStateError)
	if !ok {
		return false
	}
	*target = *stateErr
	return true
}

func (r *InferenceIdentityBindingReconciler) updateSuccessStatus(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	identities []renderedIdentity,
) error {
	return r.patchStatus(ctx, binding, func(status *kleymv1alpha1.InferenceIdentityBindingStatus) {
		status.ComputedSpiffeIDs = make([]kleymv1alpha1.ComputedSpiffeIDStatus, 0, len(identities))
		status.RenderedSelectors = make([]kleymv1alpha1.RenderedSelectorStatus, 0, len(identities))

		for _, identity := range identities {
			status.ComputedSpiffeIDs = append(status.ComputedSpiffeIDs, kleymv1alpha1.ComputedSpiffeIDStatus{
				Mode:     identity.Mode,
				SpiffeID: identity.SpiffeID,
			})
			status.RenderedSelectors = append(status.RenderedSelectors, kleymv1alpha1.RenderedSelectorStatus{
				SpiffeID:  identity.SpiffeID,
				Selectors: identity.Selectors,
			})
		}

		setCondition(status, binding.Generation, conditionTypeReady, metav1.ConditionTrue, "Reconciled", "Binding reconciled")
		setCondition(status, binding.Generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", noIdentityCollisionMessage)
		setCondition(status, binding.Generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
		setCondition(status, binding.Generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
		setCondition(status, binding.Generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
	})
}

func (r *InferenceIdentityBindingReconciler) updateFailureStatus(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	stateErr *reconcileStateError,
) error {
	return r.patchStatus(ctx, binding, func(status *kleymv1alpha1.InferenceIdentityBindingStatus) {
		status.ComputedSpiffeIDs = nil
		status.RenderedSelectors = nil

		setCondition(status, binding.Generation, conditionTypeReady, metav1.ConditionFalse, stateErr.reason, stateErr.message)
		setCondition(status, binding.Generation, stateErr.conditionType, metav1.ConditionTrue, stateErr.reason, stateErr.message)

		if stateErr.conditionType != conditionTypeInvalidRef {
			setCondition(status, binding.Generation, conditionTypeInvalidRef, metav1.ConditionFalse, "Resolved", "References are valid")
		}
		if stateErr.conditionType != conditionTypeConflict {
			setCondition(status, binding.Generation, conditionTypeConflict, metav1.ConditionFalse, "Resolved", noIdentityCollisionMessage)
		}
		if stateErr.conditionType != conditionTypeUnsafeSelector {
			setCondition(status, binding.Generation, conditionTypeUnsafeSelector, metav1.ConditionFalse, "Resolved", "Selectors are safe")
		}
		if stateErr.conditionType != conditionTypeRenderFailure {
			setCondition(status, binding.Generation, conditionTypeRenderFailure, metav1.ConditionFalse, "Resolved", "Rendering is healthy")
		}
	})
}

func setCondition(
	status *kleymv1alpha1.InferenceIdentityBindingStatus,
	generation int64,
	conditionType string,
	conditionStatus metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	})
}

func (r *InferenceIdentityBindingReconciler) patchStatus(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	mutate func(status *kleymv1alpha1.InferenceIdentityBindingStatus),
) error {
	base := binding.DeepCopy()
	mutate(&binding.Status)
	if reflect.DeepEqual(base.Status, binding.Status) {
		return nil
	}

	return r.Status().Patch(ctx, binding, client.MergeFrom(base))
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

func (r *InferenceIdentityBindingReconciler) mapObjectiveToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		binding := &bindingList.Items[i]
		if binding.Spec.TargetRef.Name != object.GetName() {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: binding.Namespace,
				Name:      binding.Name,
			},
		})
	}

	return requests
}

func (r *InferenceIdentityBindingReconciler) mapPoolToBindings(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	namespace := object.GetNamespace()
	poolName := object.GetName()
	poolGroup := object.GetObjectKind().GroupVersionKind().Group
	objectiveNames := r.objectiveNamesReferencingPool(ctx, namespace, poolName, poolGroup)
	if len(objectiveNames) == 0 {
		return nil
	}

	bindingList := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(namespace)); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(bindingList.Items))
	for i := range bindingList.Items {
		binding := &bindingList.Items[i]
		if _, exists := objectiveNames[binding.Spec.TargetRef.Name]; !exists {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: binding.Namespace,
				Name:      binding.Name,
			},
		})
	}

	return requests
}

func (r *InferenceIdentityBindingReconciler) objectiveNamesReferencingPool(
	ctx context.Context,
	namespace string,
	poolName string,
	poolGroup string,
) map[string]struct{} {
	objectiveNames := map[string]struct{}{}

	for _, gvk := range inferenceObjectiveGVKs {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			continue
		}

		for i := range list.Items {
			objective := &list.Items[i]
			ref, err := extractPoolRef(objective, namespace)
			if err != nil {
				continue
			}
			if ref.Name != poolName {
				continue
			}
			if poolGroup != "" && ref.Group != "" && ref.Group != poolGroup {
				continue
			}
			objectiveNames[objective.GetName()] = struct{}{}
		}
	}

	return objectiveNames
}
