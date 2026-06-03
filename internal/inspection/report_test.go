package inspection

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBindingInspectionReportJSONMinimalShape(t *testing.T) {
	report := NewBindingInspectionReport()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	wantTopLevelKeys := []string{
		"bindingRef",
		"capabilities",
		"desired",
		"findings",
		"generatedAt",
		"kind",
		"observed",
		"resolvedInput",
		"schemaVersion",
	}
	if keys := sortedKeys(got); !reflect.DeepEqual(keys, wantTopLevelKeys) {
		t.Fatalf("unexpected top-level keys (-want +got):\nwant %v\ngot  %v", wantTopLevelKeys, keys)
	}

	if got["schemaVersion"] != BindingInspectionReportSchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %#v", BindingInspectionReportSchemaVersion, got["schemaVersion"])
	}
	if got["kind"] != BindingInspectionReportKind {
		t.Fatalf("expected kind %q, got %#v", BindingInspectionReportKind, got["kind"])
	}
	if got["generatedAt"] != "" {
		t.Fatalf("expected empty generatedAt, got %#v", got["generatedAt"])
	}
	for _, key := range []string{"bindingRef", "resolvedInput", "desired", "observed", "capabilities"} {
		section, ok := got[key].(map[string]any)
		if !ok {
			t.Fatalf("expected %s to encode as object, got %#v", key, got[key])
		}
		if len(section) != 0 {
			t.Fatalf("expected empty %s object, got %#v", key, section)
		}
	}
	findings, ok := got["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", got["findings"])
	}
	if len(findings) != 0 {
		t.Fatalf("expected empty findings array, got %#v", findings)
	}
}

func TestBindingInspectionReportJSONRepresentativeShape(t *testing.T) {
	fallbackFalse := false
	report := BindingInspectionReport{
		GeneratedAt: "2026-05-18T09:10:11Z",
		BindingRef: BindingInspectionBindingRef{
			Namespace:  "tenant-a",
			Name:       "binding-a",
			Generation: 7,
			Mode:       "PerObjective",
			PoolRef: &BindingInspectionTargetRef{
				Name:  "pool-a",
				Group: "inference.networking.k8s.io",
			},
			ObjectiveRef: &BindingInspectionTargetRef{
				Name:  "objective-a",
				Group: "inference.networking.k8s.io",
			},
			Conditions: []metav1.Condition{{
				Type:    "Ready",
				Status:  metav1.ConditionTrue,
				Reason:  "Rendered",
				Message: "binding rendered successfully",
			}},
		},
		Resolved: BindingInspectionResolvedInput{
			PoolRef: &BindingInspectionTargetRef{
				Namespace: "tenant-a",
				Name:      "pool-a",
				Group:     "inference.networking.k8s.io",
				Version:   "v1",
				Kind:      "InferencePool",
			},
			ObjectiveRef: &BindingInspectionTargetRef{
				Namespace: "tenant-a",
				Name:      "objective-a",
				Group:     "inference.networking.k8s.io",
				Version:   "v1",
				Kind:      "InferenceObjective",
			},
			ServedGVKs: []BindingInspectionGVK{
				{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"},
				{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferenceObjective"},
			},
			PoolSelector: map[string]any{
				"matchLabels": map[string]any{"app": "pool-a"},
			},
			ContainerName: "model-server",
			SelectorProvenance: &BindingInspectionSelectorProvenance{
				PoolDerivedSelectors: []string{"k8s:pod-label:app:pool-a"},
				ContainerSelector:    "k8s:container-name:model-server",
				SafetySelectors:      []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			},
		},
		Desired: BindingInspectionDesiredState{
			ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
			SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
			PodSelector: map[string]any{
				"matchLabels": map[string]any{"app": "pool-a"},
			},
			WorkloadSelectors: []string{
				"k8s:ns:tenant-a",
				"k8s:sa:model-sa",
				"k8s:pod-label:app:pool-a",
				"k8s:container-name:model-server",
			},
			SelectorProvenance: &BindingInspectionSelectorProvenance{
				PoolDerivedSelectors: []string{"k8s:pod-label:app:pool-a"},
				ContainerSelector:    "k8s:container-name:model-server",
				SafetySelectors:      []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			},
			Hint:      "tenant-a/binding-a",
			ClassName: "kleym",
			Fallback:  &fallbackFalse,
		},
		Observed: BindingInspectionObservedState{
			ManagedClusterSPIFFEIDs: []BindingInspectionManagedClusterSPIFFEID{{
				Name:     "tenant-a-binding-a-1234abcd",
				SPIFFEID: "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
				PodSelector: map[string]any{
					"matchLabels": map[string]any{"app": "pool-a"},
				},
				WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
				Hint:              "tenant-a/binding-a",
				ClassName:         "kleym",
				Fallback:          &fallbackFalse,
			}},
			Drift: []BindingInspectionDriftEntry{{
				Field:    "spec.workloadSelectorTemplates",
				Desired:  "k8s:container-name:model-server",
				Observed: "k8s:container-name:old-server",
			}},
			EligibleWorkloads: []BindingInspectionEligibleWorkload{{
				Namespace: "tenant-a",
				Pod:       "pool-a-5f9488d7fb-q1w2e",
				Container: "model-server",
			}},
		},
		Findings: []BindingInspectionFinding{{
			ID:       "observed-drift",
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   "SelectorDrift",
			Message:  "observed workload selectors differ from desired state",
			AffectedRefs: []BindingInspectionTargetRef{{
				Name: "tenant-a-binding-a-1234abcd",
				Kind: "ClusterSPIFFEID",
			}},
		}},
		Capabilities: BindingInspectionCapabilities{
			Binding:          BindingInspectionCapabilityFull,
			GAIEResources:    BindingInspectionCapabilityFull,
			ClusterSPIFFEIDs: BindingInspectionCapabilityFull,
			PeerBindings:     BindingInspectionCapabilityPartial,
			Pods:             BindingInspectionCapabilitySkipped,
		},
	}

	data, err := json.Marshal(normalizeBindingInspectionReport(report))
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	assertObjectKeys(t, got["bindingRef"], "conditions", "generation", "mode", "name", "namespace", "objectiveRef", "poolRef")
	assertObjectKeys(t, got["resolvedInput"], "containerName", "objectiveRef", "poolRef", "poolSelector", "selectorProvenance", "servedGVKs")
	assertObjectKeys(t, got["desired"], "className", "clusterSPIFFEIDName", "fallback", "hint", "podSelector", "selectorProvenance", "spiffeID", "workloadSelectors")
	assertObjectKeys(t, got["observed"], "drift", "eligibleWorkloads", "managedClusterSPIFFEIDs")
	assertObjectKeys(t, got["capabilities"], "binding", "clusterspiffeids", "gaieResources", "peerBindings", "pods")

	resolved := got["resolvedInput"].(map[string]any)
	assertObjectKeys(t, resolved["selectorProvenance"], "containerSelector", "poolDerivedSelectors", "safetySelectors")
}

func TestBindingInspectionFindingJSONFields(t *testing.T) {
	data, err := json.Marshal(normalizeBindingInspectionReport(BindingInspectionReport{
		Findings: []BindingInspectionFinding{{
			ID:       "rbac-limited",
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   "Forbidden",
			Message:  "pods are not readable",
			AffectedRefs: []BindingInspectionTargetRef{{
				Name:      "pods",
				Namespace: "tenant-a",
			}},
		}},
	}))
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	findings := got["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", findings)
	}
	assertObjectKeys(t, findings[0], "affectedRefs", "id", "message", "reason", "severity")
}

func TestBindingInspectionReportNormalizesNilSlices(t *testing.T) {
	data, err := json.Marshal(normalizeBindingInspectionReport(BindingInspectionReport{
		Findings: []BindingInspectionFinding{{
			ID:       "binding-not-found",
			Severity: BindingInspectionFindingSeverityError,
			Reason:   "NotFound",
			Message:  "binding not found",
		}},
	}))
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	findings := got["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", findings)
	}

	finding := findings[0].(map[string]any)
	affectedRefs, ok := finding["affectedRefs"].([]any)
	if !ok {
		t.Fatalf("expected affectedRefs array, got %#v", finding["affectedRefs"])
	}
	if len(affectedRefs) != 0 {
		t.Fatalf("expected empty affectedRefs, got %#v", affectedRefs)
	}
}

func TestWriteBindingInspectionReportJSONReturnsWriterError(t *testing.T) {
	wantErr := errors.New("write failed")
	err := writeBindingInspectionReportJSON(errWriter{err: wantErr}, NewBindingInspectionReport())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected writer error %v, got %v", wantErr, err)
	}
}

func TestWriteBindingInspectionReportJSONWritesCompactJSONWithNewline(t *testing.T) {
	var out bytes.Buffer
	if err := writeBindingInspectionReportJSON(&out, NewBindingInspectionReport()); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if got := out.String(); got == "" || got[len(got)-1] != '\n' {
		t.Fatalf("expected newline-terminated JSON, got %q", got)
	}
	if bytes.Contains(out.Bytes(), []byte("\n\n")) {
		t.Fatalf("expected compact JSON, got %q", out.String())
	}
}

func TestWriteBindingInspectionReportTextRepresentativeReport(t *testing.T) {
	fallbackFalse := false
	report := BindingInspectionReport{
		GeneratedAt: "2026-05-18T09:10:11Z",
		BindingRef: BindingInspectionBindingRef{
			Namespace:  "tenant-a",
			Name:       "binding-a",
			Generation: 7,
			Mode:       "PerObjective",
			PoolRef: &BindingInspectionTargetRef{
				Namespace: "tenant-a",
				Name:      "pool-a",
				Group:     "inference.networking.k8s.io",
				Version:   "v1",
				Kind:      "InferencePool",
			},
			ObjectiveRef: &BindingInspectionTargetRef{
				Namespace: "tenant-a",
				Name:      "objective-a",
				Group:     "inference.networking.k8s.io",
				Version:   "v1",
				Kind:      "InferenceObjective",
			},
		},
		Desired: BindingInspectionDesiredState{
			ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
			SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
			PodSelector:         map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors:   []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			Hint:                "tenant-a/binding-a",
			Fallback:            &fallbackFalse,
		},
		Observed: BindingInspectionObservedState{
			ManagedClusterSPIFFEIDs: []BindingInspectionManagedClusterSPIFFEID{{
				Name:              "tenant-a-binding-a-1234abcd",
				SPIFFEID:          "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
				PodSelector:       map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
				WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
				Hint:              "tenant-a/binding-a",
				Fallback:          &fallbackFalse,
			}},
			Drift: []BindingInspectionDriftEntry{{
				Field:    "spec.workloadSelectorTemplates",
				Desired:  "k8s:container-name:model-server",
				Observed: "k8s:container-name:old-server",
			}},
			EligibleWorkloads: []BindingInspectionEligibleWorkload{{
				Namespace: "tenant-a",
				Pod:       "pool-a-12345",
				Container: "model-server",
			}},
		},
		Findings: []BindingInspectionFinding{{
			ID:       findingObservedDrift,
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   reasonObservedDrift,
			Message:  "observed managed ClusterSPIFFEID state differs from desired state",
		}},
		Capabilities: BindingInspectionCapabilities{
			Binding:          BindingInspectionCapabilityFull,
			GAIEResources:    BindingInspectionCapabilityFull,
			ClusterSPIFFEIDs: BindingInspectionCapabilityFull,
			PeerBindings:     BindingInspectionCapabilityPartial,
			Pods:             BindingInspectionCapabilitySkipped,
		},
	}

	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, report); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"BindingInspectionReport",
		"GeneratedAt: 2026-05-18T09:10:11Z",
		"Summary:",
		"Status: Warning",
		"Findings: 1",
		"Drift: 1",
		"Eligible workloads: 1",
		"Inspection completeness: partial",
		"Partial checks:\n    - Peer bindings",
		"Name: tenant-a/binding-a",
		"PoolRef: inference.networking.k8s.io/v1/InferencePool tenant-a/pool-a",
		"Identity:",
		"ClusterSPIFFEID: tenant-a-binding-a-1234abcd",
		"Selectors:",
		"Pod selector: {\"matchLabels\":{\"app\":\"pool-a\"}}",
		"Observed:",
		"Managed ClusterSPIFFEIDs: 1",
		"Status: drift detected",
		"Field: workloadSelectorTemplates",
		"Desired: k8s:container-name:model-server",
		"Observed: k8s:container-name:old-server",
		"Eligible workloads:",
		"- tenant-a/pool-a-12345 container=model-server",
		"Findings:",
		"Severity: warning",
		"Reason: ObservedDrift",
		"Message: observed managed ClusterSPIFFEID state differs from desired state",
		"Capabilities:",
		"Peer bindings: partial",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
}

func TestWriteBindingInspectionReportTextHealthyPartialSummary(t *testing.T) {
	fallbackFalse := false
	report := BindingInspectionReport{
		GeneratedAt: "2026-05-18T09:10:11Z",
		BindingRef: BindingInspectionBindingRef{
			Namespace: "tenant-a",
			Name:      "binding-a",
			Mode:      "PerObjective",
		},
		Desired: BindingInspectionDesiredState{
			ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
			SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
			PodSelector:         map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors:   []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			Hint:                "tenant-a/binding-a",
			Fallback:            &fallbackFalse,
		},
		Observed: BindingInspectionObservedState{
			ManagedClusterSPIFFEIDs: []BindingInspectionManagedClusterSPIFFEID{{
				Name: "tenant-a-binding-a-1234abcd",
			}},
			EligibleWorkloads: []BindingInspectionEligibleWorkload{{
				Namespace: "tenant-a",
				Pod:       "pool-a-12345",
				Container: "model-server",
			}},
		},
		Capabilities: BindingInspectionCapabilities{
			Binding:          BindingInspectionCapabilityFull,
			GAIEResources:    BindingInspectionCapabilityFull,
			ClusterSPIFFEIDs: BindingInspectionCapabilityFull,
			PeerBindings:     BindingInspectionCapabilityPartial,
			Pods:             BindingInspectionCapabilityFull,
		},
	}

	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, report); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Summary:\n  Status: OK",
		"Findings: none",
		"Drift: none",
		"Eligible workloads: 1",
		"Inspection completeness: partial",
		"Partial checks:\n    - Peer bindings",
		"Status: matches desired",
		"Findings:\n  none",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"SPIFFE ID: (none)",
		"Pod selector: (none)",
		"Workload selectors:\n              none",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("text output unexpectedly contains %q\n%s", unwanted, text)
		}
	}
}

func TestWriteBindingInspectionReportTextConditionDetails(t *testing.T) {
	report := BindingInspectionReport{
		BindingRef: BindingInspectionBindingRef{
			Namespace: "tenant-a",
			Name:      "binding-a",
			Conditions: []metav1.Condition{{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "RenderFailed",
				Message: "failed to render ClusterSPIFFEID because objectiveRef is missing",
			}, {
				Type:   "Conflict",
				Status: metav1.ConditionFalse,
				Reason: "NoIdentityCollision",
			}},
		},
	}

	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, report); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Ready=False",
		"Reason: RenderFailed",
		"Message: failed to render ClusterSPIFFEID because objectiveRef is missing",
		"Conflict=False",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Reason: NoIdentityCollision") {
		t.Fatalf("healthy Conflict=False condition should stay compact:\n%s", text)
	}
}

func TestWriteBindingInspectionReportTextEmptyFindings(t *testing.T) {
	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, NewBindingInspectionReport()); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Findings:\n  none\n") {
		t.Fatalf("expected empty findings text, got:\n%s", got)
	}
}

func TestWriteBindingInspectionReportTextReturnsWriterError(t *testing.T) {
	wantErr := errors.New("write failed")
	err := WriteBindingInspectionReport(errWriter{err: wantErr}, outputText, NewBindingInspectionReport())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected writer error %v, got %v", wantErr, err)
	}
}

type errWriter struct {
	err error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func sortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func assertObjectKeys(t *testing.T, value any, want ...string) {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %#v", value)
	}
	if got := sortedKeys(object); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected object keys (-want +got):\nwant %v\ngot  %v", want, got)
	}
}
