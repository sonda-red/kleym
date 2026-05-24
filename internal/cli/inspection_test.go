package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kleymv1alpha1 "github.com/sonda-red/kleym/api/v1alpha1"
	"github.com/sonda-red/kleym/internal/identity"
)

func TestInspectBindingJSONUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a"}
	newBindingInspectionRunner = func(_ *Options) (bindingInspectionRunner, error) {
		return fixedInspectionRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "binding-a", "-n", "tenant-a", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect binding returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}

	var got BindingInspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal inspect output: %v\n%s", err, stdout.String())
	}
	if got.BindingRef.Namespace != "tenant-a" || got.BindingRef.Name != "binding-a" {
		t.Fatalf("bindingRef = %#v, want tenant-a/binding-a", got.BindingRef)
	}
}

func TestInspectBindingDefaultTextUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a", Mode: "PerObjective"}
	fakeReport.Desired = BindingInspectionDesiredState{
		ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
		SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
	}
	newBindingInspectionRunner = func(_ *Options) (bindingInspectionRunner, error) {
		return fixedInspectionRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "binding-a", "-n", "tenant-a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect binding returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"BindingInspectionReport",
		"Name: tenant-a/binding-a",
		"Mode: PerObjective",
		"ClusterSPIFFEID: tenant-a-binding-a-1234abcd",
		"SPIFFE ID: spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
		"Findings:\n  none",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q\n%s", want, out)
		}
	}
}

func TestInspectionExitCodeMapping(t *testing.T) {
	setupErr := errors.New("load Kubernetes config")
	tests := []struct {
		name       string
		report     BindingInspectionReport
		inspectErr error
		strict     bool
		wantCode   int
		wantErr    error
	}{
		{
			name:     "success",
			report:   NewBindingInspectionReport(),
			wantCode: exitOK,
		},
		{
			name: "warning is non-fatal without strict",
			report: BindingInspectionReport{Findings: []BindingInspectionFinding{{
				ID:       findingObservedDrift,
				Severity: BindingInspectionFindingSeverityWarning,
				Reason:   reasonObservedDrift,
				Message:  "drift",
			}}},
			wantCode: exitOK,
		},
		{
			name: "warning is inspection issue with strict",
			report: BindingInspectionReport{Findings: []BindingInspectionFinding{{
				ID:       findingObservedDrift,
				Severity: BindingInspectionFindingSeverityWarning,
				Reason:   reasonObservedDrift,
				Message:  "drift",
			}}},
			strict:   true,
			wantCode: exitInspectionIssue,
			wantErr:  errInspectBindingHasWarningFindings,
		},
		{
			name: "error finding",
			report: BindingInspectionReport{Findings: []BindingInspectionFinding{{
				ID:       findingDependencyMissing,
				Severity: BindingInspectionFindingSeverityError,
				Reason:   "Missing",
				Message:  "missing",
			}}},
			inspectErr: errBindingInspectionErrorFindings,
			wantCode:   exitInspectionIssue,
			wantErr:    errInspectBindingHasErrorFindings,
		},
		{
			name:       "binding not found",
			report:     NewBindingInspectionReport(),
			inspectErr: errBindingInspectionNotFound,
			wantCode:   exitBindingNotFound,
			wantErr:    errBindingInspectionNotFound,
		},
		{
			name:       "inspection setup failure",
			report:     NewBindingInspectionReport(),
			inspectErr: setupErr,
			wantCode:   exitUsage,
			wantErr:    setupErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotErr := inspectionExitCode(tt.report, tt.inspectErr, tt.strict)
			if gotCode != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", gotCode, tt.wantCode)
			}
			if tt.wantErr == nil && gotErr != nil {
				t.Fatalf("expected no error, got %v", gotErr)
			}
			if tt.wantErr != nil && !errors.Is(gotErr, tt.wantErr) {
				t.Fatalf("error = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestInspectBindingSuccessReport(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := identity.RenderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := identity.DesiredClusterSPIFFEID(binding, rendered)

	inspector := newTestBindingInspector(t, nil, binding, pool, objective, managed)
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
	if report.Desired.ClusterSPIFFEIDName != rendered.Name {
		t.Fatalf("desired name = %q, want %q", report.Desired.ClusterSPIFFEIDName, rendered.Name)
	}
	if len(report.Observed.ManagedClusterSPIFFEIDs) != 1 {
		t.Fatalf("managed ClusterSPIFFEIDs = %d, want 1", len(report.Observed.ManagedClusterSPIFFEIDs))
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
		report.Capabilities.Pods != BindingInspectionCapabilitySkipped {
		t.Fatalf("unexpected capabilities: %#v", report.Capabilities)
	}
}

func TestInspectBindingMissingBindingReport(t *testing.T) {
	inspector := newTestBindingInspector(t, nil)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "missing")
	if !errors.Is(err, errBindingInspectionNotFound) {
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
	if !errors.Is(err, errBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingInvalidRef, BindingInspectionFindingSeverityError, "TargetPoolNotFound")
	if report.Capabilities.GAIEResources != BindingInspectionCapabilityPartial {
		t.Fatalf("gaieResources capability = %q, want partial", report.Capabilities.GAIEResources)
	}
}

func TestInspectBindingUnsafeSelectorFinding(t *testing.T) {
	binding := testInspectionBinding()
	binding.Spec.WorkloadSelectorTemplates = []string{"k8s:ns:tenant-a"}
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	inspector := newTestBindingInspector(t, nil, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, errBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingUnsafeSelector, BindingInspectionFindingSeverityError, "UnsafeSelector")
}

func TestInspectBindingObservedDriftFinding(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	rendered, err := identity.RenderIdentity(binding, objective, pool)
	if err != nil {
		t.Fatalf("render test identity: %v", err)
	}
	managed := identity.DesiredClusterSPIFFEID(binding, rendered)
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

func TestInspectBindingRBACLimitedClusterSPIFFEIDs(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: identity.ClusterSPIFFEIDGVK().Group, Resource: clusterSPIFFEIDResourceName},
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

func TestInspectBindingNoMatchClusterSPIFFEIDCapability(t *testing.T) {
	binding := testInspectionBinding()
	pool := testInspectionPool("pool-a")
	objective := testInspectionObjective("objective-a", "pool-a")
	noMatch := &meta.NoKindMatchError{
		GroupKind:        identity.ClusterSPIFFEIDGVK().GroupKind(),
		SearchedVersions: []string{identity.ClusterSPIFFEIDGVK().Version},
	}
	inspector := newTestBindingInspector(t, noMatch, binding, pool, objective)

	report, err := inspector.InspectBinding(context.Background(), "tenant-a", "binding-a")
	if !errors.Is(err, errBindingInspectionErrorFindings) {
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
	if !errors.Is(err, errBindingInspectionErrorFindings) {
		t.Fatalf("InspectBinding error = %v, want error findings", err)
	}

	assertFinding(t, report.Findings, findingKleymCollision, BindingInspectionFindingSeverityError, "IdentityCollision")
}

func newTestBindingInspector(t *testing.T, listErr error, objects ...client.Object) *bindingInspector {
	t.Helper()

	scheme := newBindingInspectionScheme()
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
	kubeClient := client.Client(baseClient)
	if listErr != nil {
		kubeClient = listErrorClient{Client: baseClient, err: listErr}
	}

	return &bindingInspector{
		client: kubeClient,
		mapper: newInspectionTestRESTMapper(
			append(identity.InferenceObjectiveGVKs(), identity.InferencePoolGVKs()...),
			append(identity.InferenceObjectiveGVKs(), identity.InferencePoolGVKs()...),
		),
		now: func() time.Time {
			return time.Date(2026, 5, 18, 10, 11, 12, 0, time.UTC)
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
				Group: identity.InferencePoolGVKs()[0].Group,
			},
			ObjectiveRef: &kleymv1alpha1.InferenceObjectiveTargetRef{
				Name:  "objective-a",
				Group: identity.InferenceObjectiveGVKs()[0].Group,
			},
			SelectorSource: kleymv1alpha1.SelectorSourceDerivedFromPool,
			WorkloadSelectorTemplates: []string{
				"k8s:ns:{{ .Namespace }}",
				"k8s:sa:model-sa",
			},
			Mode: kleymv1alpha1.InferenceIdentityBindingModePerObjective,
			ContainerDiscriminator: &kleymv1alpha1.ContainerDiscriminator{
				Type:  kleymv1alpha1.ContainerDiscriminatorTypeName,
				Value: "model-server",
			},
		},
	}
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
	pool.SetGroupVersionKind(identity.InferencePoolGVKs()[0])
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
				"group": identity.InferencePoolGVKs()[0].Group,
			},
		},
	}}
	objective.SetGroupVersionKind(identity.InferenceObjectiveGVKs()[0])
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
	err error
}

func (c listErrorClient) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	gvk := list.GetObjectKind().GroupVersionKind()
	if gvk.Group == identity.ClusterSPIFFEIDGVK().Group &&
		strings.HasPrefix(gvk.Kind, identity.ClusterSPIFFEIDGVK().Kind) {
		return c.err
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

func TestStableValueString(t *testing.T) {
	got := stableValueString(map[string]any{"b": "two", "a": "one"})
	if got != `{"a":"one","b":"two"}` {
		t.Fatalf("stableValueString map = %q", got)
	}
}

func TestStringSliceFromAnySkipsNonStrings(t *testing.T) {
	got := stringSliceFromAny([]any{"a", 1, "b"})
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("stringSliceFromAny = %#v", got)
	}
}

var _ client.Client = listErrorClient{}
var _ bindingInspectionRunner = fixedInspectionRunner{}
