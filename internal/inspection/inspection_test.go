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
	rendered, err := inspectionPlan(binding, pool)
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

	if report.SchemaVersion != BindingInspectionReportSchemaVersion {
		t.Fatalf("schemaVersion = %q", report.SchemaVersion)
	}
	if report.BindingRef.Name != "binding-a" {
		t.Fatalf("bindingRef = %#v", report.BindingRef)
	}
	expectedName := spirecm.BuildClusterSPIFFEIDName(binding.Namespace, binding.Name, rendered.SpiffeID)
	if report.RenderedClusterSPIFFEID.Name != expectedName {
		t.Fatalf("rendered ClusterSPIFFEID name = %q, want %q", report.RenderedClusterSPIFFEID.Name, expectedName)
	}
	if len(report.MatchedPods) != 1 ||
		report.MatchedPods[0].Pod != "model-server-a" ||
		report.MatchedPods[0].Container != "" {
		t.Fatalf("matched pods = %#v, want pod-level match", report.MatchedPods)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
	if report.Capabilities.Binding != BindingInspectionCapabilityFull ||
		report.Capabilities.GAIEResources != BindingInspectionCapabilityFull ||
		report.Capabilities.PeerBindings != BindingInspectionCapabilityPartial ||
		report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("unexpected capabilities: %#v", report.Capabilities)
	}
}

func TestInspectBindingPoolOnlyMatchedPod(t *testing.T) {
	binding := testInspectionPoolOnlyBinding()
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
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

	if len(report.MatchedPods) != 1 {
		t.Fatalf("matched pods = %#v, want one pod", report.MatchedPods)
	}
	workload := report.MatchedPods[0]
	if workload.Namespace != "tenant-a" || workload.Pod != "model-server-a" || workload.Container != "" {
		t.Fatalf("matched pod = %#v, want pod-level match without container", workload)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
	if report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("pods capability = %q, want full", report.Capabilities.Pods)
	}
}

func TestInspectBindingUsesStatusOperatorConfig(t *testing.T) {
	binding := testInspectionBinding()
	binding.Status.TrustDomain = "example.org"
	binding.Status.ClusterSPIFFEIDClassName = "kleym"
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlanWithTrustDomain(binding, pool, "example.org")
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "kleym")
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspectorWithIdentityConfig(t, inspectionIdentityConfig{
		trustDomain:              "example.org",
		clusterSPIFFEIDClassName: "kleym",
	}, nil, binding, pool, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if report.RenderedIdentity.SPIFFEID != "spiffe://example.org/ns/tenant-a/pool/pool-a" {
		t.Fatalf("rendered spiffeID = %q, want configured trust domain", report.RenderedIdentity.SPIFFEID)
	}
	if report.RenderedClusterSPIFFEID.ClassName != "kleym" {
		t.Fatalf("rendered className = %q, want kleym", report.RenderedClusterSPIFFEID.ClassName)
	}
	if report.IdentityConfig.TrustDomainSource != identityConfigSourceBindingStatus ||
		report.IdentityConfig.ClusterSPIFFEIDClassNameSource != identityConfigSourceBindingStatus {
		t.Fatalf("identityConfig = %#v, want bindingStatus sources", report.IdentityConfig)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestInspectBindingFlagsOverrideStatusOperatorConfig(t *testing.T) {
	binding := testInspectionBinding()
	binding.Status.TrustDomain = identity.DefaultTrustDomain
	binding.Status.ClusterSPIFFEIDClassName = ""
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlanWithTrustDomain(binding, pool, "example.org")
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "kleym")
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspectorWithIdentityConfig(t, inspectionIdentityConfig{
		trustDomain:                      "example.org",
		trustDomainOverride:              true,
		clusterSPIFFEIDClassName:         "kleym",
		clusterSPIFFEIDClassNameOverride: true,
	}, nil, binding, pool, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if report.IdentityConfig.TrustDomainSource != identityConfigSourceFlag ||
		report.IdentityConfig.ClusterSPIFFEIDClassNameSource != identityConfigSourceFlag {
		t.Fatalf("identityConfig = %#v, want flag sources", report.IdentityConfig)
	}
	if report.RenderedIdentity.SPIFFEID != "spiffe://example.org/ns/tenant-a/pool/pool-a" ||
		report.RenderedClusterSPIFFEID.ClassName != "kleym" {
		t.Fatalf("rendered output = %#v / %#v, want flag-rendered output", report.RenderedIdentity, report.RenderedClusterSPIFFEID)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestInspectBindingMissingStatusOperatorConfigUsesDefaultsAndWarns(t *testing.T) {
	binding := testInspectionBinding()
	binding.Status.TrustDomain = ""
	binding.Status.ClusterSPIFFEIDClassName = ""
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
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

	if report.IdentityConfig.TrustDomainSource != identityConfigSourceDefault ||
		report.IdentityConfig.ClusterSPIFFEIDClassNameSource != identityConfigSourceDefault {
		t.Fatalf("identityConfig = %#v, want default sources", report.IdentityConfig)
	}
	assertFinding(t, report.Findings, findingIdentityConfigMissing, BindingInspectionFindingSeverityWarning, reasonIdentityConfigMissing)
}

func TestInspectBindingStatusOperatorConfigSupportsClasslessOutput(t *testing.T) {
	binding := testInspectionPoolOnlyBinding()
	binding.Status.TrustDomain = identity.DefaultTrustDomain
	binding.Status.ClusterSPIFFEIDClassName = ""
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
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

	if report.IdentityConfig.ClusterSPIFFEIDClassNameSource != identityConfigSourceBindingStatus {
		t.Fatalf("identityConfig = %#v, want class source bindingStatus", report.IdentityConfig)
	}
	if report.RenderedClusterSPIFFEID.ClassName != "" {
		t.Fatalf("rendered className = %q, want classless", report.RenderedClusterSPIFFEID.ClassName)
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
	inspector := newTestBindingInspector(t, nil, binding, pool)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, ErrBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingRenderFailure, BindingInspectionFindingSeverityError, "InvalidServiceAccountName")
}

func TestInspectBindingIgnoresManagedClusterSPIFFEIDState(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	if err := unstructured.SetNestedField(managed.Object, "spiffe://wrong.example.test/ns/tenant-a/pool/pool-a", "spec", "spiffeIDTemplate"); err != nil {
		t.Fatalf("set mismatched spiffeIDTemplate: %v", err)
	}
	pod := testInspectionPod("model-server-a", "model-server")

	inspector := newTestBindingInspector(t, nil, binding, pool, managed, pod)
	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestInspectBindingDoesNotRequireManagedClusterSPIFFEID(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	pod := testInspectionPod("model-server-a", "model-server")
	inspector := newTestBindingInspector(t, nil, binding, pool, pod)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
	if report.RenderedClusterSPIFFEID.Name == "" {
		t.Fatalf("expected rendered ClusterSPIFFEID output")
	}
}

func TestInspectBindingZeroMatchedPodsFinding(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	inspector := newTestBindingInspector(t, nil, binding, pool, managed)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingZeroMatchedPods, BindingInspectionFindingSeverityInfo, reasonZeroMatchedPods)
	if report.Capabilities.Pods != BindingInspectionCapabilityFull {
		t.Fatalf("pods capability = %q, want full", report.Capabilities.Pods)
	}
}

func TestInspectBindingIgnoresClusterSPIFFEIDListRBAC(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	pod := testInspectionPod("model-server-a", "model-server")
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: spirecm.ClusterSPIFFEIDGVK().Group, Resource: "clusterspiffeids"},
		"",
		errors.New("denied"),
	)
	inspector := newTestBindingInspector(t, forbidden, binding, pool, pod)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertNoFinding(t, report.Findings, findingRBACLimited)
}

func TestInspectBindingRBACLimitedPods(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	rendered, err := inspectionPlan(binding, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := spirecm.DesiredClusterSPIFFEID(binding, rendered, "")
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Resource: podResourceName},
		"",
		errors.New("denied"),
	)
	inspector := newTestBindingInspectorWithPodListErr(t, forbidden, binding, pool, managed)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertFinding(t, report.Findings, findingRBACLimited, BindingInspectionFindingSeverityWarning, reasonRBACLimited)
	if report.Capabilities.Pods != BindingInspectionCapabilityPartial {
		t.Fatalf("pods capability = %q, want partial", report.Capabilities.Pods)
	}
}

func TestInspectBindingDoesNotRequireClusterSPIFFEIDCRD(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	pod := testInspectionPod("model-server-a", "model-server")
	noMatch := &meta.NoKindMatchError{
		GroupKind:        spirecm.ClusterSPIFFEIDGVK().GroupKind(),
		SearchedVersions: []string{spirecm.ClusterSPIFFEIDGVK().Version},
	}
	inspector := newTestBindingInspector(t, noMatch, binding, pool, pod)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if err != nil {
		t.Fatalf("InspectBinding returned error: %v", err)
	}

	assertNoFinding(t, report.Findings, findingDependencyMissing)
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
	inspector := newTestBindingInspector(t, nil, binding, pool)

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
	inspector.identityConfig = identityConfig
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
			gaie.InferencePoolGVKs(),
			gaie.InferencePoolGVKs(),
		),
		now: func() time.Time {
			return time.Date(2026, 5, 18, 10, 11, 12, 0, time.UTC)
		},
		identityConfig: identityConfig,
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
			ServiceAccountName: "model-sa",
		},
		Status: kleymv1alpha1.InferenceIdentityBindingStatus{
			TrustDomain:              identity.DefaultTrustDomain,
			ClusterSPIFFEIDClassName: "",
		},
	}
}

func testInspectionPoolOnlyBinding() *kleymv1alpha1.InferenceIdentityBinding {
	return testInspectionBinding()
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

func assertNoFinding(t *testing.T, findings []BindingInspectionFinding, id string) {
	t.Helper()

	for _, finding := range findings {
		if finding.ID == id {
			t.Fatalf("unexpected finding %s in %#v", id, findings)
		}
	}
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
	pool *unstructured.Unstructured,
) (identity.Plan, error) {
	return inspectionPlanWithTrustDomain(binding, pool, identity.DefaultTrustDomain)
}

func inspectionPlanWithTrustDomain(
	binding *kleymv1alpha1.InferenceIdentityBinding,
	pool *unstructured.Unstructured,
	trustDomain string,
) (identity.Plan, error) {
	poolSelector, poolDerivedSelectors, err := gaie.DeriveSelectorsFromPool(pool)
	if err != nil {
		return identity.Plan{}, err
	}
	return identity.PlanIdentity(identity.PlanInput{
		Binding:              binding,
		TrustDomain:          trustDomain,
		PoolName:             pool.GetName(),
		PodSelector:          poolSelector,
		PoolDerivedSelectors: poolDerivedSelectors,
	})
}

var _ client.Client = listErrorClient{}
var _ BindingInspector = fixedInspectionRunner{}
