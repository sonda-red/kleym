package inspection

import (
	"context"
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
	findingBindingNotFound       = "binding-not-found"
	findingInvalidRef            = "invalid-ref"
	findingDependencyMissing     = "dependency-missing"
	findingUnsafeSelector        = "unsafe-selector"
	findingRenderFailure         = "render-failure"
	findingKleymCollision        = "kleym-collision"
	findingZeroMatchedPods       = "zero-matched-pods"
	findingUnsupportedSelector   = "unsupported-selector"
	findingRBACLimited           = "rbac-limited"
	findingIdentityConfigMissing = "identity-config-undiscovered"
	reasonZeroMatchedPods        = "ZeroMatchedPods"
	reasonUnsupportedSelector    = "UnsupportedSelector"
	reasonRBACLimited            = "Forbidden"
	reasonIdentityConfigMissing  = "IdentityConfigUndiscovered"
	conditionTypeConflict        = "Conflict"
	podResourceName              = "pods"
)

const (
	identityConfigSourceBindingStatus = "bindingStatus"
	identityConfigSourceDefault       = "default"
	identityConfigSourceFlag          = "flag"
)

var (
	// ErrBindingInspectionNotFound reports a successful API lookup where the requested binding is absent.
	ErrBindingInspectionNotFound = errors.New("binding not found")
	// ErrBindingInspectionErrorFindings reports a completed inspection with error-severity findings.
	ErrBindingInspectionErrorFindings = errors.New("binding inspection report contains error findings")
)

// Config describes Kubernetes access settings for live binding inspection.
type Config struct {
	Context                          string
	Kubeconfig                       string
	Timeout                          time.Duration
	TrustDomain                      string
	TrustDomainOverride              bool
	ClusterSPIFFEIDClassName         string
	ClusterSPIFFEIDClassNameOverride bool
}

// BindingInspector inspects one binding and returns the stable report model.
type BindingInspector interface {
	InspectBinding(ctx context.Context, namespace string, name string) (BindingInspectionReport, error)
}

type bindingInspector struct {
	client         client.Client
	mapper         meta.RESTMapper
	now            func() time.Time
	identityConfig inspectionIdentityConfig
}

// NewKubernetesBindingInspector returns a read-only Kubernetes-backed binding inspector.
func NewKubernetesBindingInspector(config Config) (BindingInspector, error) {
	identityConfig, err := normalizedInspectionIdentityConfig(config)
	if err != nil {
		return nil, err
	}

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
		client:         kubeClient,
		mapper:         mapper,
		now:            time.Now,
		identityConfig: identityConfig,
	}, nil
}

type inspectionIdentityConfig struct {
	trustDomain                      string
	trustDomainOverride              bool
	clusterSPIFFEIDClassName         string
	clusterSPIFFEIDClassNameOverride bool
}

type resolvedIdentityConfig struct {
	trustDomain                    string
	trustDomainSource              string
	clusterSPIFFEIDClassName       string
	clusterSPIFFEIDClassNameSource string
	discovered                     bool
}

func normalizedInspectionIdentityConfig(config Config) (inspectionIdentityConfig, error) {
	trustDomain := config.TrustDomain
	if trustDomain == "" {
		trustDomain = identity.DefaultTrustDomain
	}
	identityConfig := inspectionIdentityConfig{
		trustDomain:                      trustDomain,
		trustDomainOverride:              config.TrustDomainOverride,
		clusterSPIFFEIDClassName:         config.ClusterSPIFFEIDClassName,
		clusterSPIFFEIDClassNameOverride: config.ClusterSPIFFEIDClassNameOverride,
	}
	if err := ValidateOperatorIdentityConfig(identityConfig.trustDomain, identityConfig.clusterSPIFFEIDClassName); err != nil {
		return inspectionIdentityConfig{}, err
	}
	return identityConfig, nil
}

// ValidateOperatorIdentityConfig rejects inspection settings that the operator would not start with.
func ValidateOperatorIdentityConfig(trustDomain string, clusterSPIFFEIDClassName string) error {
	if strings.TrimSpace(trustDomain) == "" {
		return fmt.Errorf("trustDomain must be configured before Kleym can render SPIFFE IDs")
	}
	if trustDomain != strings.TrimSpace(trustDomain) {
		return fmt.Errorf("trustDomain must not include leading or trailing whitespace")
	}
	if strings.HasPrefix(trustDomain, "spiffe://") {
		return fmt.Errorf("trustDomain must not include spiffe://")
	}
	if strings.Contains(trustDomain, "/") {
		return fmt.Errorf("trustDomain must not contain /")
	}
	if clusterSPIFFEIDClassName != strings.TrimSpace(clusterSPIFFEIDClassName) {
		return fmt.Errorf("clusterspiffeidClassName must not include leading or trailing whitespace")
	}
	return nil
}

// resolveIdentityConfig applies the CLI precedence contract from docs/spec/cli.md.
func (i *bindingInspector) resolveIdentityConfig(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	report *BindingInspectionReport,
) resolvedIdentityConfig {
	config := resolvedIdentityConfig{
		trustDomain:                    i.identityConfig.trustDomain,
		trustDomainSource:              identityConfigSourceDefault,
		clusterSPIFFEIDClassName:       i.identityConfig.clusterSPIFFEIDClassName,
		clusterSPIFFEIDClassNameSource: identityConfigSourceDefault,
	}

	if binding.Status.TrustDomain != "" {
		config.discovered = true
		config.trustDomain = binding.Status.TrustDomain
		config.trustDomainSource = identityConfigSourceBindingStatus
		config.clusterSPIFFEIDClassName = binding.Status.ClusterSPIFFEIDClassName
		config.clusterSPIFFEIDClassNameSource = identityConfigSourceBindingStatus
	}
	if i.identityConfig.trustDomainOverride {
		config.trustDomain = i.identityConfig.trustDomain
		config.trustDomainSource = identityConfigSourceFlag
	}
	if i.identityConfig.clusterSPIFFEIDClassNameOverride {
		config.clusterSPIFFEIDClassName = i.identityConfig.clusterSPIFFEIDClassName
		config.clusterSPIFFEIDClassNameSource = identityConfigSourceFlag
	}

	report.IdentityConfig = BindingInspectionIdentityConfig{
		TrustDomain:                    config.trustDomain,
		TrustDomainSource:              config.trustDomainSource,
		ClusterSPIFFEIDClassName:       config.clusterSPIFFEIDClassName,
		ClusterSPIFFEIDClassNameSource: config.clusterSPIFFEIDClassNameSource,
	}
	if !config.discovered {
		report.Findings = append(report.Findings, identityConfigUndiscoveredFinding(binding, config))
	}
	return config
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
			return normalizeBindingInspectionReport(report), ErrBindingInspectionNotFound
		}
		return report, fmt.Errorf("read InferenceIdentityBinding %q: %w", identity.NamespacedBindingKey(namespace, name), err)
	}

	report.Capabilities.Binding = BindingInspectionCapabilityFull
	report.Capabilities.PeerBindings = BindingInspectionCapabilityPartial
	report.BindingRef = bindingInspectionBindingRef(binding)
	i.addCollisionFinding(binding, &report)
	identityConfig := i.resolveIdentityConfig(binding, &report)

	rendered, renderedReady := i.inspectRenderedIdentity(ctx, binding, availableObjectiveGVKs, availablePoolGVKs, identityConfig, &report)
	i.inspectMatchedPods(ctx, binding, rendered, renderedReady, &report)

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

func (i *bindingInspector) inspectRenderedIdentity(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	availableObjectiveGVKs []schema.GroupVersionKind,
	availablePoolGVKs []schema.GroupVersionKind,
	identityConfig resolvedIdentityConfig,
	report *BindingInspectionReport,
) (identity.RenderedIdentity, bool) {
	mode := identity.EffectiveMode(binding.Spec.Mode)
	poolRef, err := gaie.BindingPoolRef(binding)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, bindingInspectionBindingTargetRef(binding), findingInvalidRef)
		return identity.RenderedIdentity{}, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, schema.GroupVersionKind{Kind: "InferencePool"})

	pool, err := gaie.ResolveInferencePool(ctx, i.client, availablePoolGVKs, poolRef)
	if err != nil {
		report.Capabilities.GAIEResources = BindingInspectionCapabilityPartial
		i.addFindingForError(report, err, *report.Resolved.PoolRef, "")
		return identity.RenderedIdentity{}, false
	}
	report.Resolved.PoolRef = poolRefToReportRef(poolRef, pool.GroupVersionKind())

	var objective *unstructured.Unstructured
	objectiveRef, hasObjectiveRef, err := gaie.BindingObjectiveRef(binding)
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
		return identity.RenderedIdentity{}, false
	}

	objectiveName := ""
	if objective != nil {
		objectiveName = objective.GetName()
	}
	rendered, err := identity.PlanIdentity(identity.PlanInput{
		Binding:              binding,
		TrustDomain:          identityConfig.trustDomain,
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
	clusterSPIFFEIDName := spirecm.BuildClusterSPIFFEIDName(binding.Namespace, binding.Name, rendered.Mode, rendered.SpiffeID)
	clusterSPIFFEIDHint := spirecm.BuildClusterSPIFFEIDHint(binding)
	clusterSPIFFEIDFallback := boolPtr(spirecm.RenderFallback())
	report.RenderedIdentity = BindingInspectionRenderedIdentity{
		SPIFFEID:           rendered.SpiffeID,
		PodSelector:        rendered.PodSelector,
		WorkloadSelectors:  append([]string(nil), rendered.Selectors...),
		SelectorProvenance: &provenance,
	}
	report.RenderedClusterSPIFFEID = BindingInspectionRenderedClusterSPIFFEID{
		Name:              clusterSPIFFEIDName,
		SPIFFEID:          rendered.SpiffeID,
		PodSelector:       rendered.PodSelector,
		WorkloadSelectors: append([]string(nil), rendered.Selectors...),
		Hint:              clusterSPIFFEIDHint,
		ClassName:         identityConfig.clusterSPIFFEIDClassName,
		Fallback:          clusterSPIFFEIDFallback,
	}
	report.Capabilities.GAIEResources = BindingInspectionCapabilityFull
	return rendered, true
}

// inspectMatchedPods reports live pod/container selector matches when identity output can be rendered.
func (i *bindingInspector) inspectMatchedPods(
	ctx context.Context,
	binding *kleymv1alpha1.InferenceIdentityBinding,
	rendered identity.RenderedIdentity,
	renderedReady bool,
	report *BindingInspectionReport,
) {
	if !renderedReady {
		return
	}

	selection := workloadSelectionFromSelectors(rendered.Selectors)
	if len(selection.UnsupportedSelectors) > 0 {
		report.Capabilities.Pods = BindingInspectionCapabilityPartial
		report.Findings = append(report.Findings, unsupportedSelectorFinding(binding, selection.UnsupportedSelectors))
		return
	}

	pods, err := i.listMatchingPods(ctx, binding, rendered)
	if err != nil {
		if apierrors.IsForbidden(err) {
			report.Capabilities.Pods = BindingInspectionCapabilityPartial
			report.Findings = append(report.Findings, BindingInspectionFinding{
				ID:           findingRBACLimited,
				Severity:     BindingInspectionFindingSeverityWarning,
				Reason:       reasonRBACLimited,
				Message:      "pods are not readable",
				AffectedRefs: []BindingInspectionTargetRef{podResourceTargetRef(binding.Namespace)},
			})
			return
		}
		report.Capabilities.Pods = BindingInspectionCapabilityUnknown
		report.Findings = append(report.Findings, BindingInspectionFinding{
			ID:           findingDependencyMissing,
			Severity:     BindingInspectionFindingSeverityWarning,
			Reason:       "PodListFailed",
			Message:      err.Error(),
			AffectedRefs: []BindingInspectionTargetRef{podResourceTargetRef(binding.Namespace)},
		})
		return
	}

	report.Capabilities.Pods = BindingInspectionCapabilityFull
	report.MatchedPods = matchedPodsFromPods(pods, selection, report)
	sort.Slice(report.MatchedPods, func(left, right int) bool {
		return matchedPodSortKey(report.MatchedPods[left]) <
			matchedPodSortKey(report.MatchedPods[right])
	})
	if len(report.MatchedPods) == 0 {
		report.Findings = append(report.Findings, zeroMatchedPodsFinding(binding))
	}
}

// listMatchingPods narrows pod reads to the rendered pod selector and binding namespace.
func (i *bindingInspector) listMatchingPods(
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

// matchedPodsFromPods evaluates live pod/container matches for the already-rendered identity selectors.
func matchedPodsFromPods(
	pods []corev1.Pod,
	selection workloadSelection,
	report *BindingInspectionReport,
) []BindingInspectionMatchedPod {
	workloads := []BindingInspectionMatchedPod{}
	for _, pod := range pods {
		if !podMatchesSelection(pod, selection) {
			continue
		}

		matchingContainers := matchingPodContainers(pod, selection)
		if selection.ContainerSelectorType == "" {
			workloads = append(workloads, BindingInspectionMatchedPod{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
			})
			continue
		}
		for _, container := range matchingContainers {
			workloads = append(workloads, BindingInspectionMatchedPod{
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
		return nil, fmt.Errorf("rendered pod selector matchLabels must be an object")
	}

	labels := make(map[string]string, len(matchLabels))
	for key, value := range matchLabels {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("rendered pod selector matchLabels[%q] must be a string", key)
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

// zeroMatchedPodsFinding records that readable pods did not match the rendered identity selectors.
func zeroMatchedPodsFinding(binding *kleymv1alpha1.InferenceIdentityBinding) BindingInspectionFinding {
	return BindingInspectionFinding{
		ID:       findingZeroMatchedPods,
		Severity: BindingInspectionFindingSeverityInfo,
		Reason:   reasonZeroMatchedPods,
		Message:  "no currently readable pods match the rendered identity selectors",
		AffectedRefs: []BindingInspectionTargetRef{{
			Namespace: binding.Namespace,
			Name:      binding.Name,
			Group:     kleymv1alpha1.GroupVersion.Group,
			Version:   kleymv1alpha1.GroupVersion.Version,
			Kind:      "InferenceIdentityBinding",
		}},
	}
}

// unsupportedSelectorFinding records rendered selectors that cannot be evaluated from pod data.
func unsupportedSelectorFinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	selectors []string,
) BindingInspectionFinding {
	return BindingInspectionFinding{
		ID:       findingUnsupportedSelector,
		Severity: BindingInspectionFindingSeverityWarning,
		Reason:   reasonUnsupportedSelector,
		Message: fmt.Sprintf(
			"matched pods cannot be fully evaluated because rendered selectors are unsupported by CLI pod inspection: %s",
			strings.Join(selectors, ", "),
		),
		AffectedRefs: []BindingInspectionTargetRef{{
			Namespace: binding.Namespace,
			Name:      binding.Name,
			Group:     kleymv1alpha1.GroupVersion.Group,
			Version:   kleymv1alpha1.GroupVersion.Version,
			Kind:      "InferenceIdentityBinding",
		}},
	}
}

// identityConfigUndiscoveredFinding records compatibility fallback for bindings
// reconciled before operator render settings were published in status.
func identityConfigUndiscoveredFinding(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	config resolvedIdentityConfig,
) BindingInspectionFinding {
	sourceSummary := "CLI defaults"
	if config.trustDomainSource == identityConfigSourceFlag ||
		config.clusterSPIFFEIDClassNameSource == identityConfigSourceFlag {
		sourceSummary = "CLI flags and defaults"
	}
	if config.trustDomainSource == identityConfigSourceFlag &&
		config.clusterSPIFFEIDClassNameSource == identityConfigSourceFlag {
		sourceSummary = "CLI flags"
	}
	return BindingInspectionFinding{
		ID:       findingIdentityConfigMissing,
		Severity: BindingInspectionFindingSeverityWarning,
		Reason:   reasonIdentityConfigMissing,
		Message: fmt.Sprintf(
			"operator config was not discovered from InferenceIdentityBinding status; using %s",
			sourceSummary,
		),
		AffectedRefs: []BindingInspectionTargetRef{bindingInspectionBindingTargetRef(binding)},
	}
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
	return BindingInspectionTargetRef{
		Namespace: binding.Namespace,
		Name:      binding.Name,
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

func matchedPodSortKey(workload BindingInspectionMatchedPod) string {
	return workload.Namespace + "/" + workload.Pod + "/" + workload.Container
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
