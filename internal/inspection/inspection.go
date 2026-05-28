package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/spirecm"
)

const (
	findingBindingNotFound      = "binding-not-found"
	findingInvalidRef           = "invalid-ref"
	findingDependencyMissing    = "dependency-missing"
	findingUnsafeSelector       = "unsafe-selector"
	findingRenderFailure        = "render-failure"
	findingKleymCollision       = "kleym-collision"
	findingZeroEligibleWorkload = "zero-eligible-workloads"
	findingUnsupportedSelector  = "unsupported-selector"
	findingObservedDrift        = "observed-drift"
	findingRBACLimited          = "rbac-limited"
	reasonZeroEligibleWorkload  = "ZeroEligibleWorkloads"
	reasonUnsupportedSelector   = "UnsupportedSelector"
	reasonObservedDrift         = "ObservedDrift"
	reasonRBACLimited           = "Forbidden"
	conditionTypeConflict       = "Conflict"
	clusterSPIFFEIDResourceName = "clusterspiffeids"
	podResourceName             = "pods"
)

var (
	// ErrBindingInspectionNotFound reports a successful API lookup where the requested binding is absent.
	ErrBindingInspectionNotFound = errors.New("binding not found")
	// ErrBindingInspectionErrorFindings reports a completed inspection with error-severity findings.
	ErrBindingInspectionErrorFindings = errors.New("binding inspection report contains error findings")
)

// Config describes Kubernetes access settings for live binding inspection.
type Config struct {
	Context    string
	Kubeconfig string
	Timeout    time.Duration
}

// BindingInspector inspects one binding and returns the stable report model.
type BindingInspector interface {
	InspectBinding(ctx context.Context, namespace string, name string) (BindingInspectionReport, error)
}

type bindingInspector struct {
	client client.Client
	mapper meta.RESTMapper
	now    func() time.Time
}

// NewKubernetesBindingInspector returns a read-only Kubernetes-backed binding inspector.
func NewKubernetesBindingInspector(config Config) (BindingInspector, error) {
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: config.Context}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if config.Kubeconfig != "" {
		loadingRules.ExplicitPath = config.Kubeconfig
	}

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load Kubernetes config: %w", err)
	}
	restConfig.Timeout = config.Timeout

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
	_ = corev1.AddToScheme(scheme)
	_ = kleymv1alpha1.AddToScheme(scheme)
	for _, gvk := range append(gaie.InferenceObjectiveGVKs(), gaie.InferencePoolGVKs()...) {
		registerInspectionUnstructuredGVK(scheme, gvk)
	}
	registerInspectionUnstructuredGVK(scheme, spirecm.ClusterSPIFFEIDGVK())
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
			report.Findings = append(report.Findings, inspectionFinding(
				findingBindingNotFound,
				BindingInspectionFindingSeverityError,
				string(metav1.StatusReasonNotFound),
				fmt.Sprintf("InferenceIdentityBinding %q was not found", identity.NamespacedBindingKey(namespace, name)),
				bindingInspectionTargetRef(namespace, name),
			))
			return normalizeBindingInspectionReport(report), ErrBindingInspectionNotFound
		}
		return report, fmt.Errorf("read InferenceIdentityBinding %q: %w", identity.NamespacedBindingKey(namespace, name), err)
	}

	report.Capabilities.Binding = BindingInspectionCapabilityFull
	report.Capabilities.PeerBindings = BindingInspectionCapabilityPartial
	report.BindingRef = bindingInspectionBindingRef(binding)
	i.addCollisionFinding(binding, &report)

	rendered, desiredReady := i.inspectDesiredState(ctx, binding, availableObjectiveGVKs, availablePoolGVKs, &report)
	i.inspectObservedState(ctx, binding, rendered, desiredReady, &report)
	i.inspectEligibleWorkloads(ctx, binding, rendered, desiredReady, &report)

	report = normalizeBindingInspectionReport(report)
	if HasErrorSeverityFinding(report.Findings) {
		return report, ErrBindingInspectionErrorFindings
	}
	return report, nil
}

func (i *bindingInspector) discoverGAIEGVKs() ([]schema.GroupVersionKind, []schema.GroupVersionKind, error) {
	availableObjectiveGVKs, err := filterAvailableInspectionGVKs(i.mapper, gaie.InferenceObjectiveGVKs())
	if err != nil {
		return nil, nil, err
	}
	availablePoolGVKs, err := filterAvailableInspectionGVKs(i.mapper, gaie.InferencePoolGVKs())
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
	poolRef, err := gaie.BindingPoolRef(binding)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), findingInvalidRef)
		return nil, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, schema.GroupVersionKind{Kind: "InferencePool"})

	pool, err := gaie.ResolveInferencePool(ctx, i.client, availablePoolGVKs, poolRef)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, *report.Resolved.PoolRef, "")
		return nil, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, pool.GroupVersionKind())
	return pool, true
}

	var objective *unstructured.Unstructured
	objectiveRef, hasObjectiveRef, err := gaie.BindingObjectiveRef(binding)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), findingInvalidRef)
		return nil, false
	}
	if mode == kleymv1alpha1.InferenceIdentityBindingModePerObjective && !hasObjectiveRef {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, &identity.StateError{
			ConditionType: identity.ConditionTypeRenderFailure,
			Reason:        "MissingObjectiveRef",
			Message:       "objectiveRef is required when mode is PerObjective",
		}, bindingInspectionBindingTargetRef(binding), "")
		return nil, false
	}
	if hasObjectiveRef {
		report.Resolved.ObjectiveRef = objectiveRefToReportRef(objectiveRef, schema.GroupVersionKind{Kind: "InferenceObjective"})
		objective, err = gaie.ResolveInferenceObjective(ctx, i.client, availableObjectiveGVKs, objectiveRef)
		if err != nil {
			report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
			i.addFindingForError(report, err, *report.Resolved.ObjectiveRef, "")
			return identity.RenderedIdentity{}, false
		}
		report.Resolved.ObjectiveRef = objectiveRefToReportRef(objectiveRef, objective.GroupVersionKind())
		if err := gaie.ValidateObjectiveTargetsPool(objective, pool, binding.Namespace); err != nil {
			report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
			i.addFindingForError(report, &gaie.StateError{
				ConditionType: gaie.ConditionTypeInvalidRef,
				Reason:        "InvalidObjectiveRef",
				Message:       err.Error(),
			}, *report.Resolved.ObjectiveRef, "")
			return identity.RenderedIdentity{}, false
		}
	}

	poolSelector, poolDerivedSelectors, err := gaie.DeriveSelectorsFromPool(pool)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, &identity.StateError{
			ConditionType: identity.ConditionTypeUnsafeSelector,
			Reason:        "InvalidPoolSelector",
			Message:       err.Error(),
		}, *report.Resolved.PoolRef, "")
		return false
	}

	objectiveName := ""
	if objective != nil {
		objectiveName = objective.GetName()
	}
	rendered, err := identity.PlanIdentity(identity.PlanInput{
		Binding:              binding,
		ObjectiveName:        objectiveName,
		PoolName:             pool.GetName(),
		PodSelector:          poolSelector,
		PoolDerivedSelectors: poolDerivedSelectors,
	})
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), "")
		return identity.RenderedIdentity{}, false
	}

	provenance := selectorProvenance(binding, rendered, poolDerivedSelectors)
	report.Resolved.PoolSelector = poolSelector
	report.Resolved.ContainerName = binding.Spec.ContainerName
	report.Resolved.SelectorProvenance = &provenance
	report.Desired = BindingInspectionDesiredState{
		ClusterSPIFFEIDName: spirecm.BuildClusterSPIFFEIDName(binding.Namespace, binding.Name, rendered.Mode, rendered.SpiffeID),
		SPIFFEID:            rendered.SpiffeID,
		PodSelector:         rendered.PodSelector,
		WorkloadSelectors:   append([]string(nil), rendered.Selectors...),
		SelectorProvenance:  &provenance,
		Hint:                spirecm.BuildClusterSPIFFEIDHint(binding),
		Fallback:            boolPtr(spirecm.RenderFallback()),
	}
	report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
	return true
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
					Group: spirecm.ClusterSPIFFEIDGVK().Group,
					Kind:  spirecm.ClusterSPIFFEIDGVK().Kind,
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
	recordObservedDrift(binding, rendered, objects, report)
}

// recordClusterSPIFFEIDListFailure maps list errors to the inspection capability and finding contract.
func recordClusterSPIFFEIDListFailure(err error, report *BindingInspectionReport) {
	switch {
	case meta.IsNoMatchError(err):
		report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityUnknown
		report.Findings = append(report.Findings, inspectionFinding(
			findingDependencyMissing,
			BindingInspectionFindingSeverityError,
			"ClusterSPIFFEIDCRDMissing",
			"ClusterSPIFFEID CRD is not installed",
			BindingInspectionTargetRef{
				Name:  clusterSPIFFEIDResourceName,
				Group: identity.ClusterSPIFFEIDGVK().Group,
				Kind:  identity.ClusterSPIFFEIDGVK().Kind,
			},
		))
	case apierrors.IsForbidden(err):
		report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityPartial
		report.Findings = append(report.Findings, inspectionFinding(
			findingRBACLimited,
			BindingInspectionFindingSeverityWarning,
			reasonRBACLimited,
			"ClusterSPIFFEID resources are not readable",
			BindingInspectionTargetRef{Name: clusterSPIFFEIDResourceName},
		))
	default:
		report.Capabilities.ClusterSPIFFEIDs = BindingInspectionCapabilityUnknown
		report.Findings = append(report.Findings, inspectionFinding(
			findingDependencyMissing,
			BindingInspectionFindingSeverityError,
			"ClusterSPIFFEIDListFailed",
			err.Error(),
			BindingInspectionTargetRef{Name: clusterSPIFFEIDResourceName},
		))
	}
}

	desired := spirecm.DesiredClusterSPIFFEID(binding, rendered)
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
				Name:  desired.GetName(),
				Group: spirecm.ClusterSPIFFEIDGVK().Group,
				Kind:  spirecm.ClusterSPIFFEIDGVK().Kind,
			}},
		})
	}
}

// inspectEligibleWorkloads reports live pod/container selector matches when desired state can be rendered.
func (i *bindingInspector) inspectEligibleWorkloads(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	rendered identity.RenderedIdentity,
	desiredReady bool,
	report *BindingInspectionReport,
) {
	if !desiredReady {
		return
	}

	selection := workloadSelectionFromSelectors(rendered.Selectors)
	if len(selection.UnsupportedSelectors) > 0 {
		report.Capabilities.Pods = BindingInspectionCapabilityPartial
		report.Findings = append(report.Findings, unsupportedSelectorFinding(binding, selection.UnsupportedSelectors))
		return
	}

	pods, err := i.listEligiblePods(ctx, binding, rendered)
	if err != nil {
		recordPodListFailure(err, binding.Namespace, report)
		return
	}

	report.Capabilities.Pods = BindingInspectionCapabilityFull
	report.Observed.EligibleWorkloads = eligibleWorkloadsFromPods(pods, selection, report)
	sort.Slice(report.Observed.EligibleWorkloads, func(left, right int) bool {
		return eligibleWorkloadSortKey(report.Observed.EligibleWorkloads[left]) <
			eligibleWorkloadSortKey(report.Observed.EligibleWorkloads[right])
	})
	if len(report.Observed.EligibleWorkloads) == 0 {
		report.Findings = append(report.Findings, zeroEligibleWorkloadsFinding(binding))
	}
}

// recordPodListFailure maps pod read failures to partial or unknown workload inspection.
func recordPodListFailure(err error, namespace string, report *BindingInspectionReport) {
	if apierrors.IsForbidden(err) {
		report.Capabilities.Pods = BindingInspectionCapabilityPartial
		report.Findings = append(report.Findings, inspectionFinding(
			findingRBACLimited,
			BindingInspectionFindingSeverityWarning,
			reasonRBACLimited,
			"pods are not readable",
			podResourceTargetRef(namespace),
		))
		return
	}
	report.Capabilities.Pods = BindingInspectionCapabilityUnknown
	report.Findings = append(report.Findings, inspectionFinding(
		findingDependencyMissing,
		BindingInspectionFindingSeverityWarning,
		"PodListFailed",
		err.Error(),
		podResourceTargetRef(namespace),
	))
}

// listEligiblePods narrows pod reads to the rendered pod selector and binding namespace.
func (i *bindingInspector) listEligiblePods(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	rendered identity.RenderedIdentity,
) ([]corev1.Pod, error) {
	matchLabels, err := podSelectorMatchLabels(rendered.PodSelector)
	if err != nil {
		return nil, err
	}

	podList := &corev1.PodList{}
	if err := i.client.List(
		ctx,
		podList,
		client.InNamespace(binding.Namespace),
		client.MatchingLabels(matchLabels),
	); err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func (i *bindingInspector) listManagedClusterSPIFFEIDs(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
) ([]*unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	gvk := spirecm.ClusterSPIFFEIDGVK()
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := i.client.List(ctx, list, client.MatchingLabels(spirecm.ManagedClusterSPIFFEIDLabels(binding))); err != nil {
		return nil, err
	}

	items := make([]*unstructured.Unstructured, 0, len(list.Items))
	for item := range list.Items {
		items = append(items, list.Items[item].DeepCopy())
	}
	return items, nil
}

// eligibleWorkloadsFromPods evaluates live pod/container matches for the already-rendered identity selectors.
func eligibleWorkloadsFromPods(
	pods []corev1.Pod,
	selection workloadSelection,
	report *BindingInspectionReport,
) []BindingInspectionEligibleWorkload {
	workloads := []BindingInspectionEligibleWorkload{}
	for _, pod := range pods {
		if !podMatchesSelection(pod, selection) {
			continue
		}

		matchingContainers := matchingPodContainers(pod, selection)
		if selection.ContainerSelectorType == "" {
			workloads = append(workloads, BindingInspectionEligibleWorkload{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
			})
			continue
		}
		for _, container := range matchingContainers {
			workloads = append(workloads, BindingInspectionEligibleWorkload{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
				Container: container.Name,
			})
		}
	}
	return workloads
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

// inspectionFinding keeps report findings consistent without repeating the stable JSON shape.
func inspectionFinding(
	id string,
	severity BindingInspectionFindingSeverity,
	reason string,
	message string,
	refs ...BindingInspectionTargetRef,
) BindingInspectionFinding {
	return BindingInspectionFinding{
		ID:           id,
		Severity:     severity,
		Reason:       reason,
		Message:      message,
		AffectedRefs: refs,
	}
}

func findingForError(err error, ref BindingInspectionTargetRef, fallbackID string) BindingInspectionFinding {
	var stateErr *identity.StateError
	if errors.As(err, &stateErr) {
		return inspectionFinding(
			findingIDForStateError(stateErr, fallbackID),
			BindingInspectionFindingSeverityError,
			stateErr.Reason,
			stateErr.Message,
			ref,
		)
	}
	var gaieErr *gaie.StateError
	if errors.As(err, &gaieErr) {
		return BindingInspectionFinding{
			ID:           findingIDForGAIEStateError(gaieErr, fallbackID),
			Severity:     BindingInspectionFindingSeverityError,
			Reason:       gaieErr.Reason,
			Message:      gaieErr.Message,
			AffectedRefs: []BindingInspectionTargetRef{ref},
		}
	}
	if apierrors.IsForbidden(err) {
		return inspectionFinding(
			findingRBACLimited,
			BindingInspectionFindingSeverityError,
			reasonRBACLimited,
			err.Error(),
			ref,
		)
	}
	if fallbackID == "" {
		fallbackID = findingDependencyMissing
	}
	return inspectionFinding(
		fallbackID,
		BindingInspectionFindingSeverityError,
		"InspectionFailed",
		err.Error(),
		ref,
	)
}

func findingIDForStateError(err *identity.StateError, fallbackID string) string {
	if strings.HasSuffix(err.Reason, "CRDMissing") {
		return findingDependencyMissing
	}
	switch err.ConditionType {
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

func findingIDForGAIEStateError(err *gaie.StateError, fallbackID string) string {
	if strings.HasSuffix(err.Reason, "CRDMissing") {
		return findingDependencyMissing
	}
	switch err.ConditionType {
	case gaie.ConditionTypeInvalidRef:
		return findingInvalidRef
	default:
		if fallbackID != "" {
			return fallbackID
		}
		return findingDependencyMissing
	}
}

type workloadSelection struct {
	Namespace             string
	ServiceAccount        string
	PodLabels             map[string]string
	ContainerSelectorType string
	ContainerValue        string
	UnsupportedSelectors  []string
}

// workloadSelectionFromSelectors extracts the Kubernetes selectors the CLI can evaluate from rendered SPIRE selectors.
func workloadSelectionFromSelectors(selectors []string) workloadSelection {
	selection := workloadSelection{PodLabels: map[string]string{}}
	for _, selector := range selectors {
		switch {
		case strings.HasPrefix(selector, "k8s:ns:"):
			selection.Namespace = strings.TrimPrefix(selector, "k8s:ns:")
		case strings.HasPrefix(selector, "k8s:sa:"):
			selection.ServiceAccount = strings.TrimPrefix(selector, "k8s:sa:")
		case strings.HasPrefix(selector, "k8s:container-name:"):
			selection.ContainerSelectorType = "name"
			selection.ContainerValue = strings.TrimPrefix(selector, "k8s:container-name:")
		case strings.HasPrefix(selector, "k8s:pod-label:"):
			key, value, ok := strings.Cut(strings.TrimPrefix(selector, "k8s:pod-label:"), ":")
			if ok && key != "" {
				selection.PodLabels[key] = value
			} else {
				selection.UnsupportedSelectors = append(selection.UnsupportedSelectors, selector)
			}
		default:
			selection.UnsupportedSelectors = append(selection.UnsupportedSelectors, selector)
		}
	}
	return selection
}

// podMatchesSelection checks namespace, service account, and pod-label selector requirements.
func podMatchesSelection(pod corev1.Pod, selection workloadSelection) bool {
	if selection.Namespace != "" && pod.Namespace != selection.Namespace {
		return false
	}
	if selection.ServiceAccount != "" && pod.Spec.ServiceAccountName != selection.ServiceAccount {
		return false
	}
	for key, value := range selection.PodLabels {
		if pod.Labels[key] != value {
			return false
		}
	}
	return true
}

// matchingPodContainers returns containers selected by the rendered container name.
func matchingPodContainers(pod corev1.Pod, selection workloadSelection) []corev1.Container {
	if selection.ContainerSelectorType == "" {
		return nil
	}

	containers := []corev1.Container{}
	for _, container := range pod.Spec.Containers {
		switch selection.ContainerSelectorType {
		case "name":
			if container.Name == selection.ContainerValue {
				containers = append(containers, container)
			}
		}
	}
	return containers
}

// podSelectorMatchLabels converts the rendered label selector into client-go list labels.
func podSelectorMatchLabels(selector map[string]any) (map[string]string, error) {
	rawMatchLabels, found := selector["matchLabels"]
	if !found {
		rawMatchLabels = selector
	}
	matchLabels, ok := rawMatchLabels.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("desired pod selector matchLabels must be an object")
	}

	labels := make(map[string]string, len(matchLabels))
	for key, value := range matchLabels {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("desired pod selector matchLabels[%q] must be a string", key)
		}
		labels[key] = text
	}
	return labels, nil
}

func (i *bindingInspector) addCollisionFinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	report *BindingInspectionReport,
) {
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeConflict)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		return
	}
	report.Findings = append(report.Findings, inspectionFinding(
		findingKleymCollision,
		BindingInspectionFindingSeverityError,
		condition.Reason,
		condition.Message,
		bindingInspectionBindingTargetRef(binding),
	))
}

// observedDriftFinding records that at least one visible managed object differs from desired state.
func observedDriftFinding(rendered identity.RenderedIdentity) BindingInspectionFinding {
	return inspectionFinding(
		findingObservedDrift,
		BindingInspectionFindingSeverityWarning,
		reasonObservedDrift,
		"observed managed ClusterSPIFFEID state differs from desired state",
		BindingInspectionTargetRef{
			Name:  rendered.Name,
			Group: identity.ClusterSPIFFEIDGVK().Group,
			Kind:  identity.ClusterSPIFFEIDGVK().Kind,
		},
	)
}

// zeroEligibleWorkloadsFinding records that readable pods did not match the rendered identity selectors.
func zeroEligibleWorkloadsFinding(binding *kleymv1alpha1.InferenceIdentityBinding) BindingInspectionFinding {
	return inspectionFinding(
		findingZeroEligibleWorkload,
		BindingInspectionFindingSeverityInfo,
		reasonZeroEligibleWorkload,
		"no currently readable pods match the rendered identity selectors",
		bindingInspectionBindingTargetRef(binding),
	)
}

// unsupportedSelectorFinding records rendered selectors that cannot be evaluated from pod data.
func unsupportedSelectorFinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	selectors []string,
) BindingInspectionFinding {
	return inspectionFinding(
		findingUnsupportedSelector,
		BindingInspectionFindingSeverityWarning,
		reasonUnsupportedSelector,
		fmt.Sprintf(
			"pod eligibility cannot be fully evaluated because rendered selectors are unsupported by CLI pod inspection: %s",
			strings.Join(selectors, ", "),
		),
		bindingInspectionBindingTargetRef(binding),
	)
}

func podResourceTargetRef(namespace string) BindingInspectionTargetRef {
	return BindingInspectionTargetRef{
		Namespace: namespace,
		Name:      podResourceName,
		Version:   "v1",
		Kind:      "Pod",
	}
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
	return bindingInspectionTargetRef(binding.Namespace, binding.Name)
}

func bindingInspectionTargetRef(namespace string, name string) BindingInspectionTargetRef {
	return BindingInspectionTargetRef{
		Namespace: namespace,
		Name:      name,
		Group:     kleymv1alpha1.GroupVersion.Group,
		Version:   kleymv1alpha1.GroupVersion.Version,
		Kind:      "InferenceIdentityBinding",
	}
}

func poolRefToReportRef(ref gaie.PoolRef, gvk schema.GroupVersionKind) *BindingInspectionTargetRef {
	return &BindingInspectionTargetRef{
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Group:     firstNonEmpty(gvk.Group, ref.Group),
		Version:   gvk.Version,
		Kind:      firstNonEmpty(gvk.Kind, "InferencePool"),
	}
}

func objectiveRefToReportRef(ref gaie.ObjectiveRef, gvk schema.GroupVersionKind) *BindingInspectionTargetRef {
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
	if binding.Spec.ContainerName != "" {
		if selector, err := identity.SelectorForContainerName(binding.Spec.ContainerName); err == nil {
			containerSelector = selector
		}
	}

	poolDerived := setFromStrings(poolDerivedSelectors)
	safetySelectors := make([]string, 0, 2)
	for _, selector := range rendered.Selectors {
		if selector == containerSelector {
			continue
		}
		if _, found := poolDerived[selector]; found {
			continue
		}
		if strings.HasPrefix(selector, "k8s:ns:") || strings.HasPrefix(selector, "k8s:sa:") {
			safetySelectors = append(safetySelectors, selector)
		}
	}
	sort.Strings(poolDerivedSelectors)
	sort.Strings(safetySelectors)

	return BindingInspectionSelectorProvenance{
		PoolDerivedSelectors: append([]string(nil), poolDerivedSelectors...),
		ContainerSelector:    containerSelector,
		SafetySelectors:      safetySelectors,
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
	entries := make([]BindingInspectionDriftEntry, 0)
	for _, entry := range spirecm.DriftEntries(desired, observed) {
		entries = append(entries, BindingInspectionDriftEntry{
			Field:    entry.Field,
			Desired:  entry.Desired,
			Observed: entry.Observed,
		})
	}
	return entries
}

// HasErrorSeverityFinding reports whether findings contain any error-severity item.
func HasErrorSeverityFinding(findings []BindingInspectionFinding) bool {
	for _, finding := range findings {
		if finding.Severity == BindingInspectionFindingSeverityError {
			return true
		}
	}
	return false
}

// HasWarningSeverityFinding reports whether findings contain any warning-severity item.
func HasWarningSeverityFinding(findings []BindingInspectionFinding) bool {
	for _, finding := range findings {
		if finding.Severity == BindingInspectionFindingSeverityWarning {
			return true
		}
	}
	return false
}

func eligibleWorkloadSortKey(workload BindingInspectionEligibleWorkload) string {
	return workload.Namespace + "/" + workload.Pod + "/" + workload.Container
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
