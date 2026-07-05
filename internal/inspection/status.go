package inspection

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/spirecm"
)

const (
	findingOperatorUnavailable  = "operator-unavailable"
	findingCRDMissing           = "crd-missing"
	findingBindingUnhealthy     = "binding-unhealthy"
	reasonOperatorNotFound      = "OperatorNotFound"
	reasonOperatorUnavailable   = "OperatorUnavailable"
	reasonKleymCRDMissing       = "InferenceIdentityBindingCRDMissing"
	reasonGAIECRDMissing        = "GAIECRDMissing"
	operatorLabelName           = "kleym"
	operatorLabelComponent      = "operator"
	conditionTypeReady          = "Ready"
	conditionTypeInvalidRef     = "InvalidRef"
	conditionTypeUnsafeSelector = "UnsafeSelector"
	conditionTypeRenderFailure  = "RenderFailure"
)

var (
	// ErrStatusReportErrorFindings reports a completed status evaluation with error-severity findings.
	ErrStatusReportErrorFindings = errors.New("status report contains error findings")
)

// StatusInspector builds a cluster-level status report.
type StatusInspector interface {
	Status(ctx context.Context) (StatusReport, error)
}

type statusInspector struct {
	client client.Client
	mapper meta.RESTMapper
	now    func() time.Time
}

// NewKubernetesStatusInspector returns a read-only Kubernetes-backed status inspector.
func NewKubernetesStatusInspector(config Config) (StatusInspector, error) {
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
	_ = appsv1.AddToScheme(scheme)
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

	return &statusInspector{
		client: kubeClient,
		mapper: mapper,
		now:    time.Now,
	}, nil
}

// Status aggregates Kubernetes-visible Kleym installation and binding state.
func (i *statusInspector) Status(ctx context.Context) (StatusReport, error) {
	report := NewStatusReport()
	report.GeneratedAt = i.now().UTC().Format(time.RFC3339)

	if err := i.inspectCRDs(&report); err != nil {
		return report, err
	}
	if err := i.inspectOperator(ctx, &report); err != nil {
		return report, err
	}
	if report.Components.KleymCRDs.Status == StatusResultError {
		i.finishStatusReport(&report)
		return report, ErrStatusReportErrorFindings
	}

	bindings := &kleymv1alpha1.InferenceIdentityBindingList{}
	if err := i.client.List(ctx, bindings); err != nil {
		return report, fmt.Errorf("list InferenceIdentityBindings: %w", err)
	}

	inspectBindings(bindings.Items, &report)
	i.finishStatusReport(&report)
	if HasErrorSeverityFinding(report.Findings) {
		return report, ErrStatusReportErrorFindings
	}
	return report, nil
}

func (i *statusInspector) inspectCRDs(report *StatusReport) error {
	kleymCRDAvailable, err := i.crdAvailable(kleymv1alpha1.GroupVersion.WithKind("InferenceIdentityBinding"))
	if err != nil {
		return err
	}
	if kleymCRDAvailable {
		report.Components.KleymCRDs = KleymAPIStatus{
			Status:                   StatusResultOK,
			InferenceIdentityBinding: kleymv1alpha1.GroupVersion.Version,
		}
	} else {
		report.Components.KleymCRDs = KleymAPIStatus{Status: StatusResultError, Message: "missing InferenceIdentityBinding"}
		report.Findings = append(report.Findings, crdMissingFinding(reasonKleymCRDMissing, "InferenceIdentityBinding CRD is not installed"))
	}

	spireCRDAvailable, err := i.crdAvailable(spirecm.ClusterSPIFFEIDGVK())
	if err != nil {
		return err
	}
	if spireCRDAvailable {
		report.Components.SPIRECRDs = SPIRECRDStatus{
			Status:          StatusResultOK,
			ClusterSPIFFEID: spirecm.ClusterSPIFFEIDGVK().Version,
		}
	} else {
		report.Components.SPIRECRDs = SPIRECRDStatus{Status: StatusResultError, Message: "missing ClusterSPIFFEID"}
		report.Findings = append(report.Findings, crdMissingFinding("ClusterSPIFFEIDCRDMissing", "ClusterSPIFFEID CRD is not installed"))
	}

	poolVersions, err := i.availableVersions(gaie.InferencePoolGVKs())
	if err != nil {
		return err
	}
	if len(poolVersions) > 0 {
		report.Components.GAIECRDs = GAIEStatus{
			Status:        StatusResultOK,
			InferencePool: strings.Join(poolVersions, ","),
		}
		return nil
	}
	report.Components.GAIECRDs = GAIEStatus{
		Status:        StatusResultError,
		Message:       "missing InferencePool",
		InferencePool: "unavailable",
	}
	report.Findings = append(report.Findings, crdMissingFinding(reasonGAIECRDMissing, "InferencePool CRD is not installed"))
	return nil
}

func (i *statusInspector) crdAvailable(gvk schema.GroupVersionKind) (bool, error) {
	_, err := i.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return true, nil
	}
	if meta.IsNoMatchError(err) {
		return false, nil
	}
	return false, fmt.Errorf("resolve REST mapping for %s: %w", gvk.String(), err)
}

func (i *statusInspector) availableVersions(gvks []schema.GroupVersionKind) ([]string, error) {
	versions := []string{}
	for _, gvk := range gvks {
		available, err := i.crdAvailable(gvk)
		if err != nil {
			return nil, err
		}
		if available {
			versions = append(versions, gvk.Version)
		}
	}
	return versions, nil
}

func (i *statusInspector) inspectOperator(ctx context.Context, report *StatusReport) error {
	deployments := &appsv1.DeploymentList{}
	if err := i.client.List(ctx, deployments, client.MatchingLabels{
		"app.kubernetes.io/name":      operatorLabelName,
		"app.kubernetes.io/component": operatorLabelComponent,
	}); err != nil {
		return fmt.Errorf("list Kleym operator deployments: %w", err)
	}

	if len(deployments.Items) == 0 {
		report.Components.Operator = OperatorStatus{Status: StatusResultError, Message: "not found"}
		report.Findings = append(report.Findings, BindingInspectionFinding{
			ID:       findingOperatorUnavailable,
			Severity: BindingInspectionFindingSeverityError,
			Reason:   reasonOperatorNotFound,
			Message:  "Kleym operator Deployment was not found",
		})
		return nil
	}

	report.Components.Operator = operatorStatusFromDeployment(&deployments.Items[0], StatusResultError, "no ready replicas")
	for _, deployment := range deployments.Items {
		if deployment.Status.ReadyReplicas > 0 {
			report.Components.Operator = operatorStatusFromDeployment(&deployment, StatusResultOK, "")
			fillConfigFromDeployment(&deployment, report)
			return nil
		}
	}

	report.Findings = append(report.Findings, BindingInspectionFinding{
		ID:       findingOperatorUnavailable,
		Severity: BindingInspectionFindingSeverityError,
		Reason:   reasonOperatorUnavailable,
		Message:  "Kleym operator Deployment has no ready replicas",
	})
	return nil
}

func operatorStatusFromDeployment(deployment *appsv1.Deployment, status StatusResult, message string) OperatorStatus {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	if deployment.Status.ReadyReplicas > replicas {
		replicas = deployment.Status.ReadyReplicas
	}
	return OperatorStatus{
		Status:        status,
		Message:       message,
		Deployment:    deployment.Namespace + "/" + deployment.Name,
		ReadyReplicas: deployment.Status.ReadyReplicas,
		Replicas:      replicas,
		Version:       operatorVersion(deployment),
	}
}

func operatorVersion(deployment *appsv1.Deployment) string {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != operatorLabelComponent {
			continue
		}
		return imageTag(container.Image)
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return ""
	}
	return imageTag(deployment.Spec.Template.Spec.Containers[0].Image)
}

func imageTag(image string) string {
	if image == "" {
		return ""
	}
	if digestIndex := strings.LastIndex(image, "@"); digestIndex >= 0 {
		return image[digestIndex+1:]
	}
	slashIndex := strings.LastIndex(image, "/")
	colonIndex := strings.LastIndex(image, ":")
	if colonIndex > slashIndex {
		return image[colonIndex+1:]
	}
	return ""
}

func fillConfigFromDeployment(deployment *appsv1.Deployment, report *StatusReport) {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != operatorLabelComponent {
			continue
		}
		fillConfigFromArgs(container.Args, report)
		return
	}
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		fillConfigFromArgs(deployment.Spec.Template.Spec.Containers[0].Args, report)
	}
}

func fillConfigFromArgs(args []string, report *StatusReport) {
	trustDomainFound := false
	className := ""
	classNameFound := false
	for _, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--trust-domain="); ok && report.Config.TrustDomain == "" {
			report.Config.TrustDomain = value
			trustDomainFound = true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--clusterspiffeid-class-name="); ok {
			className = value
			classNameFound = true
		}
	}
	if !report.Config.ClusterSPIFFEIDClassNameKnown && (trustDomainFound || classNameFound) {
		report.Config.ClusterSPIFFEIDClassName = className
		report.Config.ClusterSPIFFEIDClassNameKnown = true
	}
}

type statusConfigObservations struct {
	trustDomains map[string]struct{}
	classNames   map[string]struct{}
}

func newStatusConfigObservations() statusConfigObservations {
	return statusConfigObservations{
		trustDomains: map[string]struct{}{},
		classNames:   map[string]struct{}{},
	}
}

func (o statusConfigObservations) addBinding(binding *kleymv1alpha1.InferenceIdentityBinding) {
	if binding.Status.TrustDomain == "" {
		return
	}
	o.trustDomains[binding.Status.TrustDomain] = struct{}{}
	o.classNames[binding.Status.ClusterSPIFFEIDClassName] = struct{}{}
}

func fillConfigFromBindingObservations(observations statusConfigObservations, report *StatusReport) {
	if report.Config.TrustDomain == "" {
		if value, ok := observedConfigValue(observations.trustDomains); ok {
			report.Config.TrustDomain = value
		}
	}
	if !report.Config.ClusterSPIFFEIDClassNameKnown {
		if value, ok := observedConfigValue(observations.classNames); ok {
			report.Config.ClusterSPIFFEIDClassName = value
			report.Config.ClusterSPIFFEIDClassNameKnown = true
		}
	}
}

func observedConfigValue(values map[string]struct{}) (string, bool) {
	switch len(values) {
	case 0:
		return "", false
	case 1:
		for value := range values {
			return value, true
		}
	}
	return "mixed", true
}

func addBindingConditionCounts(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	summary *BindingConditionSummary,
) {
	if conditionIsTrue(binding, conditionTypeReady) {
		summary.Ready++
	}
	if conditionIsTrue(binding, conditionTypeConflict) {
		summary.Conflict++
	}
	if conditionIsTrue(binding, conditionTypeInvalidRef) {
		summary.InvalidRef++
	}
	if conditionIsTrue(binding, conditionTypeUnsafeSelector) {
		summary.UnsafeSelector++
	}
	if conditionIsTrue(binding, conditionTypeRenderFailure) {
		summary.RenderFailure++
	}
}

func conditionIsTrue(binding *kleymv1alpha1.InferenceIdentityBinding, conditionType string) bool {
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func inspectBindings(bindings []kleymv1alpha1.InferenceIdentityBinding, report *StatusReport) {
	configObservations := newStatusConfigObservations()

	for index := range bindings {
		binding := &bindings[index]
		report.Summary.Bindings.Total++
		addBindingConditionCounts(binding, &report.Summary.Bindings.Conditions)
		configObservations.addBinding(binding)
		switch bindingResult(binding) {
		case StatusResultOK:
			report.Summary.Bindings.OK++
		case StatusResultWarning:
			report.Summary.Bindings.Warning++
		default:
			report.Summary.Bindings.Error++
			report.Findings = append(report.Findings, bindingUnhealthyFinding(binding))
		}

	}

	fillConfigFromBindingObservations(configObservations, report)
}

func bindingResult(binding *kleymv1alpha1.InferenceIdentityBinding) StatusResult {
	ready := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeReady)
	if ready == nil || ready.Status == metav1.ConditionUnknown {
		return StatusResultWarning
	}
	if ready.Status == metav1.ConditionTrue {
		return StatusResultOK
	}
	return StatusResultError
}

func (i *statusInspector) finishStatusReport(report *StatusReport) {
	if HasErrorSeverityFinding(report.Findings) {
		report.Status = StatusResultError
		report.Components.Kleym = ComponentStatus{Status: StatusResultError}
		return
	}
	if HasWarningSeverityFinding(report.Findings) || report.Summary.Bindings.Warning > 0 {
		report.Status = StatusResultWarning
		report.Components.Kleym = ComponentStatus{Status: StatusResultWarning}
		return
	}
	report.Status = StatusResultOK
	report.Components.Kleym = ComponentStatus{Status: StatusResultOK}
}

func crdMissingFinding(reason string, message string) BindingInspectionFinding {
	return BindingInspectionFinding{
		ID:       findingCRDMissing,
		Severity: BindingInspectionFindingSeverityError,
		Reason:   reason,
		Message:  message,
	}
}

func bindingUnhealthyFinding(binding *kleymv1alpha1.InferenceIdentityBinding) BindingInspectionFinding {
	condition := meta.FindStatusCondition(binding.Status.Conditions, conditionTypeReady)
	reason := "BindingNotReady"
	message := "binding is not ready"
	if condition != nil {
		reason = condition.Reason
		message = condition.Message
	}
	return BindingInspectionFinding{
		ID:       findingBindingUnhealthy,
		Severity: BindingInspectionFindingSeverityError,
		Reason:   reason,
		Message:  message,
		AffectedRefs: []BindingInspectionTargetRef{{
			Namespace: binding.Namespace,
			Name:      binding.Name,
			Group:     kleymv1alpha1.GroupVersion.Group,
			Version:   kleymv1alpha1.GroupVersion.Version,
			Kind:      "InferenceIdentityBinding",
		}},
	}
}
