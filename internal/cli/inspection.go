package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

const (
	findingBindingNotFound      = "binding-not-found"
	findingInvalidRef           = "invalid-ref"
	findingDependencyMissing    = "dependency-missing"
	findingUnsafeSelector       = "unsafe-selector"
	findingRenderFailure        = "render-failure"
	findingKleymCollision       = "kleym-collision"
	findingObservedDrift        = "observed-drift"
	findingRBACLimited          = "rbac-limited"
	reasonObservedDrift         = "ObservedDrift"
	reasonRBACLimited           = "Forbidden"
	conditionTypeConflict       = "Conflict"
	clusterSPIFFEIDResourceName = "clusterspiffeids"
)

var (
	errBindingInspectionNotFound      = errors.New("binding not found")
	errBindingInspectionErrorFindings = errors.New("binding inspection report contains error findings")
)

type bindingInspectionRunner interface {
	InspectBinding(ctx context.Context, namespace string, name string) (BindingInspectionReport, error)
}

var newBindingInspectionRunner = newKubernetesBindingInspector

type bindingInspector struct {
	client client.Client
	mapper meta.RESTMapper
	now    func() time.Time
}

func newKubernetesBindingInspector(opts *Options) (bindingInspectionRunner, error) {
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: opts.Context}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load Kubernetes config: %w", err)
	}
	restConfig.Timeout = opts.Timeout

	scheme := newBindingInspectionScheme()
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes HTTP client: %w", err)
	}
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes REST mapper: %w", err)
	}
	kubeClient, err := client.New(restConfig, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}

	return &bindingInspector{
		client: kubeClient,
		mapper: mapper,
		now:    time.Now,
	}, nil
}

func newBindingInspectionScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = kleymv1alpha1.AddToScheme(scheme)
	for _, gvk := range append(identity.InferenceObjectiveGVKs(), identity.InferencePoolGVKs()...) {
		registerInspectionUnstructuredGVK(scheme, gvk)
	}
	registerInspectionUnstructuredGVK(scheme, identity.ClusterSPIFFEIDGVK())
	return scheme
}

func registerInspectionUnstructuredGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) {
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
}

// InspectBinding builds the stable JSON report for one InferenceIdentityBinding.
func (i *bindingInspector) InspectBinding(ctx context.Context, namespace string, name string) (BindingInspectionReport, error) {
	report := NewBindingInspectionReport()
	report.GeneratedAt = i.now().UTC().Format(time.RFC3339)
	report.BindingRef = BindingInspectionBindingRef{Namespace: namespace, Name: name}
	report.Capabilities.Pods = BindingInspectionCapabilitySkipped

	availableObjectiveGVKs, availablePoolGVKs, err := i.discoverGAIEGVKs()
	if err != nil {
		return report, fmt.Errorf("discover served GAIE resources: %w", err)
	}
	report.Resolved.ServedGVKs = bindingInspectionGVKs(append(availablePoolGVKs, availableObjectiveGVKs...))

	binding := &kleymv1alpha1.InferenceIdentityBinding{}
	if err := i.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, binding); err != nil {
		if apierrors.IsNotFound(err) {
			report.Capabilities.Binding = BindingInspectionCapabilityFull
			report.Findings = append(report.Findings, BindingInspectionFinding{
				ID:       findingBindingNotFound,
				Severity: BindingInspectionFindingSeverityError,
				Reason:   string(metav1.StatusReasonNotFound),
				Message:  fmt.Sprintf("InferenceIdentityBinding %q was not found", identity.NamespacedBindingKey(namespace, name)),
				AffectedRefs: []BindingInspectionTargetRef{{
					Namespace: namespace,
					Name:      name,
					Group:     kleymv1alpha1.GroupVersion.Group,
					Version:   kleymv1alpha1.GroupVersion.Version,
					Kind:      "InferenceIdentityBinding",
				}},
			})
			return normalizeBindingInspectionReport(report), errBindingInspectionNotFound
		}
		return report, fmt.Errorf("read InferenceIdentityBinding %q: %w", identity.NamespacedBindingKey(namespace, name), err)
	}

	report.Capabilities.Binding = BindingInspectionCapabilityFull
	report.Capabilities.PeerBindings = BindingInspectionCapabilityPartial
	report.BindingRef = bindingInspectionBindingRef(binding)
	i.addCollisionFinding(binding, &report)

	rendered, desiredReady := i.inspectDesiredState(ctx, binding, availableObjectiveGVKs, availablePoolGVKs, &report)
	i.inspectObservedState(ctx, binding, rendered, desiredReady, &report)

	report = normalizeBindingInspectionReport(report)
	if hasErrorSeverityFinding(report.Findings) {
		return report, errBindingInspectionErrorFindings
	}
	return report, nil
}

func (i *bindingInspector) discoverGAIEGVKs() ([]schema.GroupVersionKind, []schema.GroupVersionKind, error) {
	availableObjectiveGVKs, err := filterAvailableInspectionGVKs(i.mapper, identity.InferenceObjectiveGVKs())
	if err != nil {
		return nil, nil, err
	}
	availablePoolGVKs, err := filterAvailableInspectionGVKs(i.mapper, identity.InferencePoolGVKs())
	if err != nil {
		return nil, nil, err
	}
	return availableObjectiveGVKs, availablePoolGVKs, nil
}

func filterAvailableInspectionGVKs(
	mapper meta.RESTMapper,
	candidates []schema.GroupVersionKind,
) ([]schema.GroupVersionKind, error) {
	available := make([]schema.GroupVersionKind, 0, len(candidates))
	for _, gvk := range candidates {
		if _, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			return nil, fmt.Errorf("resolve REST mapping for %s: %w", gvk.String(), err)
		}
		available = append(available, gvk)
	}
	return available, nil
}

func (i *bindingInspector) inspectDesiredState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	availableObjectiveGVKs []schema.GroupVersionKind,
	availablePoolGVKs []schema.GroupVersionKind,
	report *BindingInspectionReport,
) (identity.RenderedIdentity, bool) {
	mode := identity.EffectiveMode(binding.Spec.Mode)
	poolRef, err := identity.BindingPoolRef(binding)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), findingInvalidRef)
		return identity.RenderedIdentity{}, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, schema.GroupVersionKind{Kind: "InferencePool"})

	pool, err := identity.ResolveInferencePool(ctx, i.client, availablePoolGVKs, poolRef)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, *report.Resolved.PoolRef, "")
		return identity.RenderedIdentity{}, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, pool.GroupVersionKind())

	var objective *unstructured.Unstructured
	objectiveRef, hasObjectiveRef, err := identity.BindingObjectiveRef(binding)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), findingInvalidRef)
		return identity.RenderedIdentity{}, false
	}
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective && !hasObjectiveRef {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, &identity.StateError{
			ConditionType: identity.ConditionTypeRenderFailure,
			Reason:        "MissingObjectiveRef",
			Message:       "objectiveRef is required when mode is PerObjective",
		}, bindingInspectionBindingTargetRef(binding), "")
		return identity.RenderedIdentity{}, false
	}
	if hasObjectiveRef {
		report.Resolved.ObjectiveRef = objectiveRefToReportRef(objectiveRef, schema.GroupVersionKind{Kind: "InferenceObjective"})
		objective, err = identity.ResolveInferenceObjective(ctx, i.client, availableObjectiveGVKs, objectiveRef)
		if err != nil {
			report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
			i.addFindingForError(report, err, *report.Resolved.ObjectiveRef, "")
			return identity.RenderedIdentity{}, false
		}
		report.Resolved.ObjectiveRef = objectiveRefToReportRef(objectiveRef, objective.GroupVersionKind())
		if err := identity.ValidateObjectiveTargetsPool(objective, pool, binding.Namespace); err != nil {
			report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
			i.addFindingForError(report, &identity.StateError{
				ConditionType: identity.ConditionTypeInvalidRef,
				Reason:        "InvalidObjectiveRef",
				Message:       err.Error(),
			}, *report.Resolved.ObjectiveRef, "")
			return identity.RenderedIdentity{}, false
		}
	}

	rendered, err := identity.RenderIdentity(binding, objective, pool)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), "")
		return identity.RenderedIdentity{}, false
	}

	poolSelector, poolDerivedSelectors, err := identity.DeriveSelectorsFromPool(pool)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, &identity.StateError{
			ConditionType: identity.ConditionTypeUnsafeSelector,
			Reason:        "InvalidPoolSelector",
			Message:       err.Error(),
		}, *report.Resolved.PoolRef, "")
		return identity.RenderedIdentity{}, false
	}

	provenance := selectorProvenance(binding, rendered, poolDerivedSelectors)
	report.Resolved.PoolSelector = poolSelector
	report.Resolved.ContainerDiscriminator = containerDiscriminator(binding)
	report.Resolved.SelectorProvenance = &provenance
	report.Desired = BindingInspectionDesiredState{
		ClusterSPIFFEIDName: rendered.Name,
		SPIFFEID:            rendered.SpiffeID,
		PodSelector:         rendered.PodSelector,
		WorkloadSelectors:   append([]string(nil), rendered.Selectors...),
		SelectorProvenance:  &provenance,
		Hint:                rendered.Hint,
		Fallback:            boolPtr(rendered.Fallback),
	}
	report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
	return rendered, true
}

func (i *bindingInspector) inspectObservedState(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	rendered identity.RenderedIdentity,
	desiredReady bool,
	report *BindingInspectionReport,
) {
	objects, err := i.listManagedClusterSPIFFEIDs(ctx, binding)
	if err != nil {
		switch {
		case meta.IsNoMatchError(err):
			report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityUnknown
			report.Findings = append(report.Findings, BindingInspectionFinding{
				ID:       findingDependencyMissing,
				Severity: BindingInspectionFindingSeverityError,
				Reason:   "ClusterSPIFFEIDCRDMissing",
				Message:  "ClusterSPIFFEID CRD is not installed",
				AffectedRefs: []BindingInspectionTargetRef{{
					Name:  clusterSPIFFEIDResourceName,
					Group: identity.ClusterSPIFFEIDGVK().Group,
					Kind:  identity.ClusterSPIFFEIDGVK().Kind,
				}},
			})
		case apierrors.IsForbidden(err):
			report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityPartial
			report.Findings = append(report.Findings, BindingInspectionFinding{
				ID:           findingRBACLimited,
				Severity:     BindingInspectionFindingSeverityWarning,
				Reason:       reasonRBACLimited,
				Message:      "ClusterSPIFFEID resources are not readable",
				AffectedRefs: []BindingInspectionTargetRef{{Name: clusterSPIFFEIDResourceName}},
			})
		default:
			report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityUnknown
			report.Findings = append(report.Findings, BindingInspectionFinding{
				ID:           findingDependencyMissing,
				Severity:     BindingInspectionFindingSeverityError,
				Reason:       "ClusterSPIFFEIDListFailed",
				Message:      err.Error(),
				AffectedRefs: []BindingInspectionTargetRef{{Name: clusterSPIFFEIDResourceName}},
			})
		}
		return
	}

	report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityFull
	for _, object := range objects {
		report.Observed.ManagedClusterSPIFFEIDs = append(
			report.Observed.ManagedClusterSPIFFEIDs,
			managedClusterSPIFFEIDReport(object),
		)
	}
	sort.Slice(report.Observed.ManagedClusterSPIFFEIDs, func(left, right int) bool {
		return report.Observed.ManagedClusterSPIFFEIDs[left].Name < report.Observed.ManagedClusterSPIFFEIDs[right].Name
	})

	if !desiredReady {
		return
	}

	desired := identity.DesiredClusterSPIFFEID(binding, rendered)
	if len(objects) == 0 {
		report.Observed.Drift = append(report.Observed.Drift, BindingInspectionDriftEntry{
			Field:    "metadata.name",
			Desired:  desired.GetName(),
			Observed: "",
		})
	}
	for _, object := range objects {
		report.Observed.Drift = append(report.Observed.Drift, driftEntries(desired, object)...)
	}
	if len(report.Observed.Drift) > 0 {
		sort.Slice(report.Observed.Drift, func(left, right int) bool {
			if report.Observed.Drift[left].Field == report.Observed.Drift[right].Field {
				return report.Observed.Drift[left].Observed < report.Observed.Drift[right].Observed
			}
			return report.Observed.Drift[left].Field < report.Observed.Drift[right].Field
		})
		report.Findings = append(report.Findings, BindingInspectionFinding{
			ID:       findingObservedDrift,
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   reasonObservedDrift,
			Message:  "observed managed ClusterSPIFFEID state differs from desired state",
			AffectedRefs: []BindingInspectionTargetRef{{
				Name:  rendered.Name,
				Group: identity.ClusterSPIFFEIDGVK().Group,
				Kind:  identity.ClusterSPIFFEIDGVK().Kind,
			}},
		})
	}
}

func (i *bindingInspector) listManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]*unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	gvk := identity.ClusterSPIFFEIDGVK()
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := i.client.List(ctx, list, client.MatchingLabels(identity.ManagedClusterSPIFFEIDLabels(binding))); err != nil {
		return nil, err
	}

	items := make([]*unstructured.Unstructured, 0, len(list.Items))
	for item := range list.Items {
		items = append(items, list.Items[item].DeepCopy())
	}
	return items, nil
}

func (i *bindingInspector) addFindingForError(
	report *BindingInspectionReport,
	err error,
	ref BindingInspectionTargetRef,
	fallbackID string,
) {
	finding := findingForError(err, ref, fallbackID)
	report.Findings = append(report.Findings, finding)
}

func findingForError(err error, ref BindingInspectionTargetRef, fallbackID string) BindingInspectionFinding {
	var stateErr *identity.StateError
	if errors.As(err, &stateErr) {
		return BindingInspectionFinding{
			ID:           findingIDForStateError(stateErr, fallbackID),
			Severity:     BindingInspectionFindingSeverityError,
			Reason:       stateErr.Reason,
			Message:      stateErr.Message,
			AffectedRefs: []BindingInspectionTargetRef{ref},
		}
	}
	if apierrors.IsForbidden(err) {
		return BindingInspectionFinding{
			ID:           findingRBACLimited,
			Severity:     BindingInspectionFindingSeverityError,
			Reason:       reasonRBACLimited,
			Message:      err.Error(),
			AffectedRefs: []BindingInspectionTargetRef{ref},
		}
	}
	if fallbackID == "" {
		fallbackID = findingDependencyMissing
	}
	return BindingInspectionFinding{
		ID:           fallbackID,
		Severity:     BindingInspectionFindingSeverityError,
		Reason:       "InspectionFailed",
		Message:      err.Error(),
		AffectedRefs: []BindingInspectionTargetRef{ref},
	}
}

func findingIDForStateError(err *identity.StateError, fallbackID string) string {
	if strings.HasSuffix(err.Reason, "CRDMissing") {
		return findingDependencyMissing
	}
	switch err.ConditionType {
	case identity.ConditionTypeInvalidRef:
		return findingInvalidRef
	case identity.ConditionTypeUnsafeSelector:
		return findingUnsafeSelector
	case identity.ConditionTypeRenderFailure:
		return findingRenderFailure
	default:
		if fallbackID != "" {
			return fallbackID
		}
		return findingDependencyMissing
	}
}

func (i *bindingInspector) addCollisionFinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	report *BindingInspectionReport,
) {
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeConflict)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		return
	}
	report.Findings = append(report.Findings, BindingInspectionFinding{
		ID:       findingKleymCollision,
		Severity: BindingInspectionFindingSeverityError,
		Reason:   condition.Reason,
		Message:  condition.Message,
		AffectedRefs: []BindingInspectionTargetRef{{
			Namespace: binding.Namespace,
			Name:      binding.Name,
			Group:     kleymv1alpha1.GroupVersion.Group,
			Version:   kleymv1alpha1.GroupVersion.Version,
			Kind:      "InferenceIdentityBinding",
		}},
	})
}

func bindingInspectionBindingRef(binding *kleymv1alpha1.InferenceIdentityBinding) BindingInspectionBindingRef {
	ref := BindingInspectionBindingRef{
		Namespace:  binding.Namespace,
		Name:       binding.Name,
		Generation: binding.Generation,
		Mode:       string(identity.EffectiveMode(binding.Spec.Mode)),
		PoolRef: &BindingInspectionTargetRef{
			Namespace: binding.Namespace,
			Name:      binding.Spec.PoolRef.Name,
			Group:     binding.Spec.PoolRef.Group,
			Kind:      "InferencePool",
		},
		Conditions: append([]metav1.Condition(nil), binding.Status.Conditions...),
	}
	if binding.Spec.ObjectiveRef != nil {
		ref.ObjectiveRef = &BindingInspectionTargetRef{
			Namespace: binding.Namespace,
			Name:      binding.Spec.ObjectiveRef.Name,
			Group:     binding.Spec.ObjectiveRef.Group,
			Kind:      "InferenceObjective",
		}
	}
	return ref
}

func bindingInspectionBindingTargetRef(binding *kleymv1alpha1.InferenceIdentityBinding) BindingInspectionTargetRef {
	return BindingInspectionTargetRef{
		Namespace: binding.Namespace,
		Name:      binding.Name,
		Group:     kleymv1alpha1.GroupVersion.Group,
		Version:   kleymv1alpha1.GroupVersion.Version,
		Kind:      "InferenceIdentityBinding",
	}
}

func poolRefToReportRef(ref identity.PoolRef, gvk schema.GroupVersionKind) *BindingInspectionTargetRef {
	return &BindingInspectionTargetRef{
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Group:     firstNonEmpty(gvk.Group, ref.Group),
		Version:   gvk.Version,
		Kind:      firstNonEmpty(gvk.Kind, "InferencePool"),
	}
}

func objectiveRefToReportRef(ref identity.ObjectiveRef, gvk schema.GroupVersionKind) *BindingInspectionTargetRef {
	return &BindingInspectionTargetRef{
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Group:     firstNonEmpty(gvk.Group, ref.Group),
		Version:   gvk.Version,
		Kind:      firstNonEmpty(gvk.Kind, "InferenceObjective"),
	}
}

func bindingInspectionGVKs(gvks []schema.GroupVersionKind) []BindingInspectionGVK {
	result := make([]BindingInspectionGVK, 0, len(gvks))
	for _, gvk := range gvks {
		result = append(result, BindingInspectionGVK{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})
	}
	return result
}

func selectorProvenance(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	rendered identity.RenderedIdentity,
	poolDerivedSelectors []string,
) BindingInspectionSelectorProvenance {
	containerSelector := ""
	if binding.Spec.ContainerDiscriminator != nil {
		if selector, err := identity.SelectorForContainerDiscriminator(binding.Spec.ContainerDiscriminator); err == nil {
			containerSelector = selector
		}
	}

	poolDerived := setFromStrings(poolDerivedSelectors)
	workloadSelectors := make([]string, 0, len(rendered.Selectors))
	safetySelectors := make([]string, 0, 2)
	for _, selector := range rendered.Selectors {
		if selector == containerSelector {
			continue
		}
		if _, found := poolDerived[selector]; found {
			continue
		}
		workloadSelectors = append(workloadSelectors, selector)
		if strings.HasPrefix(selector, "k8s:ns:") || strings.HasPrefix(selector, "k8s:sa:") {
			safetySelectors = append(safetySelectors, selector)
		}
	}
	sort.Strings(poolDerivedSelectors)
	sort.Strings(workloadSelectors)
	sort.Strings(safetySelectors)

	return BindingInspectionSelectorProvenance{
		SelectorSource:       string(binding.Spec.SelectorSource),
		PoolDerivedSelectors: append([]string(nil), poolDerivedSelectors...),
		WorkloadSelectors:    workloadSelectors,
		ContainerSelector:    containerSelector,
		SafetySelectors:      safetySelectors,
	}
}

func containerDiscriminator(binding *kleymv1alpha1.InferenceIdentityBinding) *BindingInspectionContainerDiscriminator {
	if binding.Spec.ContainerDiscriminator == nil {
		return nil
	}
	return &BindingInspectionContainerDiscriminator{
		Type:  string(binding.Spec.ContainerDiscriminator.Type),
		Value: binding.Spec.ContainerDiscriminator.Value,
	}
}

func managedClusterSPIFFEIDReport(object *unstructured.Unstructured) BindingInspectionManagedClusterSPIFFEID {
	spec, _, _ := unstructured.NestedMap(object.Object, "spec")
	spiffeID, _ := spec["spiffeIDTemplate"].(string)
	podSelector, _ := spec["podSelector"].(map[string]any)
	hint, _ := spec["hint"].(string)
	fallback, hasFallback := spec["fallback"].(bool)

	report := BindingInspectionManagedClusterSPIFFEID{
		Name:              object.GetName(),
		SPIFFEID:          spiffeID,
		PodSelector:       podSelector,
		WorkloadSelectors: stringSliceFromAny(spec["workloadSelectorTemplates"]),
		Hint:              hint,
		Conditions:        clusterSPIFFEIDConditions(object),
	}
	if hasFallback {
		report.Fallback = boolPtr(fallback)
	}
	return report
}

func clusterSPIFFEIDConditions(object *unstructured.Unstructured) []metav1.Condition {
	rawConditions, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil || !found {
		return nil
	}
	data, err := json.Marshal(rawConditions)
	if err != nil {
		return nil
	}
	var conditions []metav1.Condition
	if err := json.Unmarshal(data, &conditions); err != nil {
		return nil
	}
	return conditions
}

func driftEntries(desired *unstructured.Unstructured, observed *unstructured.Unstructured) []BindingInspectionDriftEntry {
	entries := []BindingInspectionDriftEntry{}
	if desired.GetName() != observed.GetName() {
		entries = append(entries, BindingInspectionDriftEntry{
			Field:    "metadata.name",
			Desired:  desired.GetName(),
			Observed: observed.GetName(),
		})
	}

	desiredSpec, _, _ := unstructured.NestedMap(desired.Object, "spec")
	observedSpec, _, _ := unstructured.NestedMap(observed.Object, "spec")
	for _, field := range []string{
		"spiffeIDTemplate",
		"podSelector",
		"workloadSelectorTemplates",
		"hint",
		"fallback",
	} {
		if reflect.DeepEqual(desiredSpec[field], observedSpec[field]) {
			continue
		}
		entries = append(entries, BindingInspectionDriftEntry{
			Field:    "spec." + field,
			Desired:  stableValueString(desiredSpec[field]),
			Observed: stableValueString(observedSpec[field]),
		})
	}
	return entries
}

func hasErrorSeverityFinding(findings []BindingInspectionFinding) bool {
	for _, finding := range findings {
		if finding.Severity == BindingInspectionFindingSeverityError {
			return true
		}
	}
	return false
}

func stableValueString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}

func setFromStrings(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolPtr(value bool) *bool {
	return &value
}
