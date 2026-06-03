package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/gaie"
	"github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/spirecm"
)

func TestInspectBindingSuccessReport(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := inspectionPlan(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspector(t, nil, binding, pool, objective, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if report.SchemaVersion != BindingInspectionReportSchemaVersion {
		t.Fatalf("schemaVersion = %q", report.SchemaVersion)
	}
	if report.BindingRef.Name != "binding-a" || report.BindingRef.Mode != string(kleymv1alpha1.InferenceIdentityBindingModePerObjective) {
		t.Fatalf("bindingRef = %#v", report.BindingRef)
	}
	expectedName := spirecm.BuildClusterSPIFFEIDName(binding.Namespace, binding.Name, rendered.Mode, rendered.SpiffeID)
	if report.Desired.ClusterSPIFFEIDName != expectedName {
		t.Fatalf("desired name = %q, want %q", report.Desired.ClusterSPIFFEIDName, expectedName)
	}
	if len(report.Observed.ManagedClusterSPIFFEIDs) != 1 {
		t.Fatalf("managed ClusterSPIFFEIDs = %d, want 1", len(report.Observed.ManagedClusterSPIFFEIDs))
	}
	if len(report.Observed.EligibleWorkloads) != 1 ||
		report.Observed.EligibleWorkloads[0].Container != "model-server" {
		t.Fatalf("eligible workloads = %#v, want model-server container", report.Observed.EligibleWorkloads)
	}
	if len(report.Observed.Drift) != 0 {
		t.Fatalf("expected no drift, got %#v", report.Observed.Drift)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
	if report.Capabilities.Binding != BindingInspectionCapabilityFull ||
		report.Capabilities.GAIEResources != BindingInspectionCapabilityFull ||
		report.Capabilities.ClusterSPIFFEIDs != BindingInspectionCapabilityFull ||
		report.Capabilities.PeerBindings != BindingInspectionCapabilityPartial ||
		report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("unexpected capabilities: %#v", report.Capabilities)
	}
}

func TestInspectBindingPoolOnlyEligibleWorkload(t *testing.T) {
	binding := testInspectionPoolOnlyBinding()
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, nil, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspector(t, nil, binding, pool, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if report.BindingRef.Mode != string(kleymv1alpha1.InferenceIdentityBindingModePoolOnly) {
		t.Fatalf("bindingRef mode = %q, want PoolOnly", report.BindingRef.Mode)
	}
	if len(report.Observed.EligibleWorkloads) != 1 {
		t.Fatalf("eligible workloads = %#v, want one pod", report.Observed.EligibleWorkloads)
	}
	workload := report.Observed.EligibleWorkloads[0]
	if workload.Namespace != "tenant-a" || workload.Pod != "model-server-a" || workload.Container != "" {
		t.Fatalf("eligible workload = %#v, want pod-level workload without container", workload)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
	if report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("pods capability = %q, want full", report.Capabilities.Pods)
	}
}

func TestInspectBindingUsesConfiguredOperatorOutput(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := inspectionPlanWithTrustDomain(binding, objective, pool, "example.org")
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "kleym")
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspectorWithIdentityConfig(t, inspectionIdentityConfig{
		trustDomain:              "example.org",
		clusterSPIFFEIDClassName: "kleym",
	}, nil, binding, pool, objective, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if report.Desired.SPIFFEID != "spiffe://example.org/ns/tenant-a/objective/objective-a" {
		t.Fatalf("desired spiffeID = %q, want configured trust domain", report.Desired.SPIFFEID)
	}
	if report.Desired.ClassName != "kleym" {
		t.Fatalf("desired className = %q, want kleym", report.Desired.ClassName)
	}
	if len(report.Observed.ManagedClusterSPIFFEIDs) != 1 ||
		report.Observed.ManagedClusterSPIFFEIDs[0].ClassName != "kleym" {
		t.Fatalf("managed ClusterSPIFFEIDs = %#v, want className kleym", report.Observed.ManagedClusterSPIFFEIDs)
	}
	if len(report.Observed.Drift) != 0 {
		t.Fatalf("expected no drift, got %#v", report.Observed.Drift)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestInspectBindingMissingBindingReport(t *testing.T) {
	inspector := newTestBindingInspector(t, nil)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "missing")
	if !errors.Is(err, ErrBindingInspectionNotFound) {
		t.Fatalf("InspectBinding error = %v, want not found", err)
	}

	assertFinding(t, report.Findings, findingBindingNotFound, BindingInspectionFindingSeverityError, string(metav1.StatusReasonNotFound))
	if report.BindingRef.Namespace != "tenant-a" || report.BindingRef.Name != "missing" {
		t.Fatalf("bindingRef = %#v, want requested binding", report.BindingRef)
	}
}

func TestInspectBindingMissingDependencyFinding(t *testing.T) {
	binding := testInspectionBinding()
	inspector := newTestBindingInspector(t, nil, binding)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, ErrBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingInvalidRef, BindingInspectionFindingSeverityError, "TargetPoolNotFound")
	if report.Capabilities.GAIEResources != BindingInspectionCapabilityPartial {
		t.Fatalf("gaieResources capability = %q, want partial", report.Capabilities.GAIEResources)
	}
}

func TestInspectBindingInvalidServiceAccountNameFinding(t *testing.T) {
	binding := testInspectionBinding()
	binding.Spec.ServiceAccountName = "Invalid_ServiceAccount"
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	inspector := newTestBindingInspector(t, nil, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, ErrBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingRenderFailure, BindingInspectionFindingSeverityError, "InvalidServiceAccountName")
}

func TestInspectBindingObservedDriftFinding(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := inspectionPlan(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	if err := unstructured.SetNestedField(managed.Object, "spiffe://drifted.example.test/ns/tenant-a/objective/objective-a", "spec", "spiffeIDTemplate"); err != nil {
		t.Fatalf("set drifted spiffeIDTemplate: %v", err)
	}

	inspector := newTestBindingInspector(t, nil, binding, pool, objective, managed)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingObservedDrift, BindingInspectionFindingSeverityWarning, reasonObservedDrift)
	if len(report.Observed.Drift) == 0 {
		t.Fatalf("expected drift entries")
	}
	if !driftContainsField(report.Observed.Drift, "spec.spiffeIDTemplate") {
		t.Fatalf("expected spiffeIDTemplate drift, got %#v", report.Observed.Drift)
	}
}

func TestInspectBindingMissingManagedOutputDrift(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	inspector := newTestBindingInspector(t, nil, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingObservedDrift, BindingInspectionFindingSeverityWarning, reasonObservedDrift)
	if !driftContainsField(report.Observed.Drift, "metadata.name") {
		t.Fatalf("expected missing managed object drift, got %#v", report.Observed.Drift)
	}
}

func TestInspectBindingZeroEligibleWorkloadsFinding(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := inspectionPlan(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	inspector := newTestBindingInspector(t, nil, binding, pool, objective, managed)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingZeroEligibleWorkload, BindingInspectionFindingSeverityInfo, reasonZeroEligibleWorkload)
	if report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("pods capability = %q, want full", report.Capabilities.Pods)
	}
}

func TestInspectBindingRBACLimitedClusterSPIFFEIDs(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: spirecm.ClusterSPIFFEIDGVK().Group, Resource: clusterSPIFFEIDResourceName},
		"",
		errors.New("denied"),
	)
	inspector := newTestBindingInspector(t, forbidden, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingRBACLimited, BindingInspectionFindingSeverityWarning, reasonRBACLimited)
	if report.Capabilities.ClusterSPIFFEIDs != BindingInspectionCapabilityPartial {
		t.Fatalf("clusterspiffeids capability = %q, want partial", report.Capabilities.ClusterSPIFFEIDs)
	}
}

func TestInspectBindingRBACLimitedPods(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := inspectionPlan(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Resource: podResourceName},
		"",
		errors.New("denied"),
	)
	inspector := newTestBindingInspectorWithPodListErr(t, forbidden, binding, pool, objective, managed)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingRBACLimited, BindingInspectionFindingSeverityWarning, reasonRBACLimited)
	if report.Capabilities.Pods != BindingInspectionCapabilityPartial {
		t.Fatalf("pods capability = %q, want partial", report.Capabilities.Pods)
	}
}

func TestInspectBindingNoMatchClusterSPIFFEIDCapability(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	noMatch := &meta.NoKindMatchError{
		GroupKind:        spirecm.ClusterSPIFFEIDGVK().GroupKind(),
		SearchedVersions: []string{spirecm.ClusterSPIFFEIDGVK().Version},
	}
	inspector := newTestBindingInspector(t, noMatch, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, ErrBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingDependencyMissing, BindingInspectionFindingSeverityError, "ClusterSPIFFEIDCRDMissing")
	if report.Capabilities.ClusterSPIFFEIDs != BindingInspectionCapabilityUnknown {
		t.Fatalf("clusterspiffeids capability = %q, want unknown", report.Capabilities.ClusterSPIFFEIDs)
	}
}

func TestInspectBindingCollisionConditionFinding(t *testing.T) {
	binding := testInspectionBinding()
	binding.Status.Conditions = []metav1.Condition{{
		Type:    conditionTypeConflict,
		Status:  metav1.ConditionTrue,
		Reason:  "IdentityCollision",
		Message: "identity collision with bindings peer-a",
	}}
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	inspector := newTestBindingInspector(t, nil, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, ErrBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingKleymCollision, BindingInspectionFindingSeverityError, "IdentityCollision")
}

func newTestBindingInspector(t *testing.T, listErr error, objects ...client.Object) *bindingInspector {
	return newTestBindingInspectorWithListErrors(t, listErr, nil, objects...)
}

func newTestBindingInspectorWithIdentityConfig(
	t *testing.T,
	identityConfig inspectionIdentityConfig,
	listErr error,
	objects ...client.Object,
) *bindingInspector {
	t.Helper()

	inspector := newTestBindingInspectorWithListErrors(t, listErr, nil, objects...)
	inspector.trustDomain = identityConfig.trustDomain
	inspector.clusterSPIFFEIDClassName = identityConfig.clusterSPIFFEIDClassName
	return inspector
}

func newTestBindingInspectorWithPodListErr(t *testing.T, podListErr error, objects ...client.Object) *bindingInspector {
	return newTestBindingInspectorWithListErrors(t, nil, podListErr, objects...)
}

func newTestBindingInspectorWithListErrors(
	t *testing.T,
	clusterSPIFFEIDListErr error,
	podListErr error,
	objects ...client.Object,
) *bindingInspector {
	t.Helper()

	scheme := newBindingInspectionScheme()
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
	kubeClient := client.Client(baseClient)
	if clusterSPIFFEIDListErr != nil || podListErr != nil {
		kubeClient = listErrorClient{
			Client:             baseClient,
			clusterSPIFFEIDErr: clusterSPIFFEIDListErr,
			podErr:             podListErr,
		}
	}

	identityConfig, err := normalizedInspectionIdentityConfig(Config{})
	if err != nil {
		t.Fatalf("default identity config: %v", err)
	}

	return &bindingInspector{
		client: kubeClient,
		mapper: newInspectionTestRESTMapper(
			append(gaie.InferenceObjectiveGVKs(), gaie.InferencePoolGVKs()...),
			append(gaie.InferenceObjectiveGVKs(), gaie.InferencePoolGVKs()...),
		),
		now: func() time.Time {
			return time.Date(2026, 5, 18, 10, 11, 12, 0, time.UTC)
		},
		trustDomain:              identityConfig.trustDomain,
		clusterSPIFFEIDClassName: identityConfig.clusterSPIFFEIDClassName,
	}
}

func testInspectionPod(name string, containers ...string) *corev1.Pod {
	podContainers := make([]corev1.Container, 0, len(containers))
	for _, container := range containers {
		containerName := container
		image := "registry.example.test/" + container + ":v1"
		if strings.Contains(container, "=") {
			var ok bool
			containerName, image, ok = strings.Cut(container, "=")
			if !ok {
				continue
			}
		}
		podContainers = append(podContainers, corev1.Container{
			Name:  containerName,
			Image: image,
		})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "tenant-a",
			Name:      name,
			Labels: map[string]string{
				"app": "model-server",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "model-sa",
			Containers:         podContainers,
		},
	}
}

func testInspectionBinding() *kleymv1alpha1.InferenceIdentityBinding {
	return &kleymv1alpha1.InferenceIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "tenant-a",
			Name:       "binding-a",
			Generation: 3,
		},
		Spec: kleymv1alpha1.InferenceIdentityBindingSpec{
			PoolRef: kleymv1alpha1.InferencePoolTargetRef{
				Name:  "pool-a",
				Group: gaie.InferencePoolGVKs()[0].Group,
			},
			ObjectiveRef: &kleymv1alpha1.InferenceObjectiveTargetRef{
				Name:  "objective-a",
				Group: gaie.InferenceObjectiveGVKs()[0].Group,
			},
			ServiceAccountName: "model-sa",
			Mode:               kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerName:      "model-server",
		},
	}
}

func testInspectionPoolOnlyBinding() *kleymv1alpha1.InferenceIdentityBinding {
	binding := testInspectionBinding()
	binding.Spec.ObjectiveRef = nil
	binding.Spec.Mode = kleymv1alpha1.InferenceIdentityBindingModePoolOnly
	binding.Spec.ContainerName = ""
	return binding
}

func testInspectionPool(name string) *unstructured.Unstructured {
	pool := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"namespace": "tenant-a",
			"name":      name,
		},
		"spec": map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "model-server",
				},
			},
		},
	}}
	pool.SetGroupVersionKind(gaie.InferencePoolGVKs()[0])
	return pool
}

func testInspectionObjective(name string, poolName string) *unstructured.Unstructured {
	objective := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"namespace": "tenant-a",
			"name":      name,
		},
		"spec": map[string]any{
			"poolRef": map[string]any{
				"name":  poolName,
				"group": gaie.InferencePoolGVKs()[0].Group,
			},
		},
	}}
	objective.SetGroupVersionKind(gaie.InferenceObjectiveGVKs()[0])
	return objective
}

func assertFinding(
	t *testing.T,
	findings []BindingInspectionFinding,
	id string,
	severity BindingInspectionFindingSeverity,
	reason string,
) {
	t.Helper()

	for _, finding := range findings {
		if finding.ID == id && finding.Severity == severity && finding.Reason == reason {
			return
		}
	}
	t.Fatalf("finding %s/%s/%s not found in %#v", id, severity, reason, findings)
}

func driftContainsField(entries []BindingInspectionDriftEntry, field string) bool {
	for _, entry := range entries {
		if entry.Field == field {
			return true
		}
	}
	return false
}

type fixedInspectionRunner struct {
	report BindingInspectionReport
	err    error
}

func (r fixedInspectionRunner) InspectBinding(_ context.Context, _ string, _ string) (BindingInspectionReport, error) {
	return r.report, r.err
}

type listErrorClient struct {
	client.Client
	clusterSPIFFEIDErr error
	podErr             error
}

func (c listErrorClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	if _, ok := list.(*corev1.PodList); ok && c.podErr != nil {
		return c.podErr
	}
	gvk := list.GetObjectKind().GroupVersionKind()
	if gvk.Group == spirecm.ClusterSPIFFEIDGVK().Group &&
		strings.HasPrefix(gvk.Kind, spirecm.ClusterSPIFFEIDGVK().Kind) &&
		c.clusterSPIFFEIDErr != nil {
		return c.clusterSPIFFEIDErr
	}
	return c.Client.List(ctx, list, opts...)
}

func newInspectionTestRESTMapper(candidates []schema.GroupVersionKind, available []schema.GroupVersionKind) meta.RESTMapper {
	versions := uniqueInspectionGroupVersions(candidates)
	mapper := meta.NewDefaultRESTMapper(versions)
	for _, gvk := range available {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}
	return mapper
}

func uniqueInspectionGroupVersions(gvks []schema.GroupVersionKind) []schema.GroupVersion {
	seen := map[schema.GroupVersion]struct{}{}
	versions := make([]schema.GroupVersion, 0, len(gvks))
	for _, gvk := range gvks {
		gv := gvk.GroupVersion()
		if _, exists := seen[gv]; exists {
			continue
		}
		seen[gv] = struct{}{}
		versions = append(versions, gv)
	}
	return versions
}

func inspectionPlan(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
) (identity.Plan, error) {
	return inspectionPlanWithTrustDomain(binding, objective, pool, identity.DefaultTrustDomain)
}

func inspectionPlanWithTrustDomain(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	objective *unstructured.Unstructured,
	pool *unstructured.Unstructured,
	trustDomain string,
) (identity.Plan, error) {
	poolSelector, poolDerivedSelectors, err := gaie.DeriveSelectorsFromPool(pool)
	if err != nil {
		return identity.Plan{}, err
	}
	objectiveName := ""
	if objective != nil {
		objectiveName = objective.GetName()
	}
	return identity.PlanIdentity(identity.PlanInput{
		Binding:              binding,
		TrustDomain:          trustDomain,
		ObjectiveName:        objectiveName,
		PoolName:             pool.GetName(),
		PodSelector:          poolSelector,
		PoolDerivedSelectors: poolDerivedSelectors,
	})
}

func TestStringSliceFromAnySkipsNonStrings(t *testing.T) {
	got := stringSliceFromAny([]any{"a", 1, "b"})
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("stringSliceFromAny = %#v", got)
	}
}

var _ client.Client = listErrorClient{}
var _ BindingInspector = fixedInspectionRunner{}
