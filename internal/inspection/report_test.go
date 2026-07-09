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
		"findings",
		"generatedAt",
		"identityConfig",
		"kind",
		"matchedPods",
		"renderedClusterSPIFFEID",
		"renderedIdentity",
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
	for _, key := range []string{"bindingRef", "identityConfig", "resolvedInput", "renderedIdentity", "renderedClusterSPIFFEID"} {
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
	matchedPods, ok := got["matchedPods"].([]any)
	if !ok {
		t.Fatalf("expected matchedPods array, got %#v", got["matchedPods"])
	}
	if len(matchedPods) != 0 {
		t.Fatalf("expected empty matchedPods array, got %#v", matchedPods)
	}
}

func TestBindingInspectionReportJSONRepresentativeShape(t *testing.T) {
	fallbackFalse := false
	report := BindingInspectionReport{
		GeneratedAt: "2026-05-18T09:10:11Z",
		IdentityConfig: BindingInspectionIdentityConfig{
			TrustDomain:                    "kleym.sonda.red",
			TrustDomainSource:              identityConfigSourceBindingStatus,
			ClusterSPIFFEIDClassName:       "kleym",
			ClusterSPIFFEIDClassNameSource: identityConfigSourceBindingStatus,
		},
		BindingRef: BindingInspectionBindingRef{
			Namespace:  "tenant-a",
			Name:       "binding-a",
			Generation: 7,
			PoolRef: &BindingInspectionTargetRef{
				Name:  "pool-a",
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
			ServedGVKs: []BindingInspectionGVK{
				{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"},
			},
			PoolSelector: map[string]any{
				"matchLabels": map[string]any{"app": "pool-a"},
			},
			SelectorProvenance: &BindingInspectionSelectorProvenance{
				PoolDerivedSelectors: []string{"k8s:pod-label:app:pool-a"},
				SafetySelectors:      []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			},
		},
		RenderedIdentity: BindingInspectionRenderedIdentity{
			SPIFFEID: "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector: map[string]any{
				"matchLabels": map[string]any{"app": "pool-a"},
			},
			WorkloadSelectors: []string{
				"k8s:ns:tenant-a",
				"k8s:sa:model-sa",
				"k8s:pod-label:app:pool-a",
			},
			SelectorProvenance: &BindingInspectionSelectorProvenance{
				PoolDerivedSelectors: []string{"k8s:pod-label:app:pool-a"},
				SafetySelectors:      []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			},
		},
		RenderedClusterSPIFFEID: BindingInspectionRenderedClusterSPIFFEID{
			Name:     "tenant-a-binding-a-1234abcd",
			SPIFFEID: "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector: map[string]any{
				"matchLabels": map[string]any{"app": "pool-a"},
			},
			WorkloadSelectors: []string{
				"k8s:ns:tenant-a",
				"k8s:sa:model-sa",
				"k8s:pod-label:app:pool-a",
			},
			Hint:      "tenant-a/binding-a",
			ClassName: "kleym",
			Fallback:  &fallbackFalse,
		},
		MatchedPods: []BindingInspectionMatchedPod{{
			Namespace: "tenant-a",
			Pod:       "pool-a-5f9488d7fb-q1w2e",
			Container: "model-server",
		}},
		Findings: []BindingInspectionFinding{{
			ID:       "rbac-limited",
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   "Forbidden",
			Message:  "pods are not readable",
			AffectedRefs: []BindingInspectionTargetRef{{
				Name: "pods",
			}},
		}},
	}

	data, err := json.Marshal(normalizeBindingInspectionReport(report))
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}

	assertObjectKeys(t, got["bindingRef"], "conditions", "generation", "name", "namespace", "poolRef")
	assertObjectKeys(t, got["identityConfig"], "clusterSPIFFEIDClassName", "clusterSPIFFEIDClassNameSource", "trustDomain", "trustDomainSource")
	assertObjectKeys(t, got["resolvedInput"], "poolRef", "poolSelector", "selectorProvenance", "servedGVKs")
	assertObjectKeys(t, got["renderedIdentity"], "podSelector", "selectorProvenance", "spiffeID", "workloadSelectors")
	assertObjectKeys(t, got["renderedClusterSPIFFEID"], "className", "fallback", "hint", "name", "podSelector", "spiffeID", "workloadSelectors")
	matchedPods := got["matchedPods"].([]any)
	if len(matchedPods) != 1 {
		t.Fatalf("matchedPods = %#v, want one pod", matchedPods)
	}

	resolved := got["resolvedInput"].(map[string]any)
	assertObjectKeys(t, resolved["selectorProvenance"], "poolDerivedSelectors", "safetySelectors")
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
			PoolRef: &BindingInspectionTargetRef{
				Namespace: "tenant-a",
				Name:      "pool-a",
				Group:     "inference.networking.k8s.io",
				Version:   "v1",
				Kind:      "InferencePool",
			},
		},
		RenderedIdentity: BindingInspectionRenderedIdentity{
			SPIFFEID:          "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector:       map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
		},
		RenderedClusterSPIFFEID: BindingInspectionRenderedClusterSPIFFEID{
			Name:              "tenant-a-binding-a-1234abcd",
			SPIFFEID:          "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector:       map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			Hint:              "tenant-a/binding-a",
			Fallback:          &fallbackFalse,
		},
		MatchedPods: []BindingInspectionMatchedPod{{
			Namespace: "tenant-a",
			Pod:       "pool-a-12345",
			Container: "model-server",
		}},
		Findings: []BindingInspectionFinding{{
			ID:       findingRBACLimited,
			Severity: BindingInspectionFindingSeverityWarning,
			Reason:   reasonRBACLimited,
			Message:  "pods are not readable",
		}},
	}

	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, report); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Binding: tenant-a/binding-a",
		"Source: InferencePool tenant-a/pool-a",
		"Identity:",
		"SPIFFE ID: spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
		"namespace: tenant-a",
		"serviceAccount: model-sa",
		"ClusterSPIFFEID:",
		"Name: tenant-a-binding-a-1234abcd",
		"Matched pods:",
		"tenant-a/pool-a-12345 container=model-server",
		"Findings:",
		"Warning Forbidden",
		"pods are not readable",
		"Exit code: 0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
}

func TestWriteBindingInspectionReportTextHealthy(t *testing.T) {
	fallbackFalse := false
	report := BindingInspectionReport{
		GeneratedAt: "2026-05-18T09:10:11Z",
		BindingRef: BindingInspectionBindingRef{
			Namespace: "tenant-a",
			Name:      "binding-a",
		},
		RenderedIdentity: BindingInspectionRenderedIdentity{
			SPIFFEID:          "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector:       map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
		},
		RenderedClusterSPIFFEID: BindingInspectionRenderedClusterSPIFFEID{
			Name:              "tenant-a-binding-a-1234abcd",
			SPIFFEID:          "spiffe://kleym.sonda.red/ns/tenant-a/sa/model-sa/inference/pool/pool-a",
			PodSelector:       map[string]any{"matchLabels": map[string]any{"app": "pool-a"}},
			WorkloadSelectors: []string{"k8s:ns:tenant-a", "k8s:sa:model-sa"},
			Hint:              "tenant-a/binding-a",
			Fallback:          &fallbackFalse,
		},
		MatchedPods: []BindingInspectionMatchedPod{{
			Namespace: "tenant-a",
			Pod:       "pool-a-12345",
			Container: "model-server",
		}},
		Capabilities: BindingInspectionCapabilities{
			Pods: BindingInspectionCapabilityFull,
		},
	}

	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, report); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Binding: tenant-a/binding-a",
		"Identity:",
		"ClusterSPIFFEID:",
		"Matched pods:",
		"tenant-a/pool-a-12345 container=model-server",
		"Findings: none",
		"Exit code: 0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"Capabilities:",
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
				Message: "failed to render ClusterSPIFFEID because serviceAccountName is invalid",
			}, {
				Type:   "InvalidRef",
				Status: metav1.ConditionFalse,
				Reason: "Resolved",
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
		"Message: failed to render ClusterSPIFFEID because serviceAccountName is invalid",
		"InvalidRef=False",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Reason: Resolved") {
		t.Fatalf("healthy InvalidRef=False condition should stay compact:\n%s", text)
	}
}

func TestWriteBindingInspectionReportTextEmptyFindings(t *testing.T) {
	var out bytes.Buffer
	if err := WriteBindingInspectionReport(&out, outputText, NewBindingInspectionReport()); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Findings: none\n") {
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
