package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// BindingInspectionReportSchemaVersion is the stable machine-readable report version from docs/spec/cli.md.
	BindingInspectionReportSchemaVersion = "v1alpha1"
	// BindingInspectionReportKind is the stable report kind from docs/spec/cli.md.
	BindingInspectionReportKind = "BindingInspectionReport"
)

const (
	BindingInspectionFindingSeverityInfo    BindingInspectionFindingSeverity = "info"
	BindingInspectionFindingSeverityWarning BindingInspectionFindingSeverity = "warning"
	BindingInspectionFindingSeverityError   BindingInspectionFindingSeverity = "error"
)

const (
	BindingInspectionCapabilityFull    BindingInspectionCapability = "full"
	BindingInspectionCapabilityPartial BindingInspectionCapability = "partial"
	BindingInspectionCapabilitySkipped BindingInspectionCapability = "skipped"
	BindingInspectionCapabilityUnknown BindingInspectionCapability = "unknown"
)

// BindingInspectionFindingSeverity is the stable severity enum for machine-readable findings.
type BindingInspectionFindingSeverity string

// BindingInspectionCapability is the stable completeness enum for machine-readable capabilities.
type BindingInspectionCapability string

// BindingInspectionReport captures the JSON contract for `kleym inspect binding -o json`.
type BindingInspectionReport struct {
	SchemaVersion string                         `json:"schemaVersion"`
	Kind          string                         `json:"kind"`
	GeneratedAt   string                         `json:"generatedAt"`
	BindingRef    BindingInspectionBindingRef    `json:"bindingRef"`
	Resolved      BindingInspectionResolvedInput `json:"resolvedInput"`
	Desired       BindingInspectionDesiredState  `json:"desired"`
	Observed      BindingInspectionObservedState `json:"observed"`
	Findings      []BindingInspectionFinding     `json:"findings"`
	Capabilities  BindingInspectionCapabilities  `json:"capabilities"`
}

// BindingInspectionBindingRef identifies the binding being inspected.
type BindingInspectionBindingRef struct {
	Namespace    string                      `json:"namespace,omitempty"`
	Name         string                      `json:"name,omitempty"`
	Generation   int64                       `json:"generation,omitempty"`
	Mode         string                      `json:"mode,omitempty"`
	PoolRef      *BindingInspectionTargetRef `json:"poolRef,omitempty"`
	ObjectiveRef *BindingInspectionTargetRef `json:"objectiveRef,omitempty"`
	Conditions   []metav1.Condition          `json:"conditions,omitempty"`
}

// BindingInspectionTargetRef is a stable compact reference shape for binding inputs.
type BindingInspectionTargetRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind,omitempty"`
}

// BindingInspectionResolvedInput captures the resolved resources and selector inputs.
type BindingInspectionResolvedInput struct {
	PoolRef                *BindingInspectionTargetRef              `json:"poolRef,omitempty"`
	ObjectiveRef           *BindingInspectionTargetRef              `json:"objectiveRef,omitempty"`
	ServedGVKs             []BindingInspectionGVK                   `json:"servedGVKs,omitempty"`
	PoolSelector           map[string]any                           `json:"poolSelector,omitempty"`
	ContainerDiscriminator *BindingInspectionContainerDiscriminator `json:"containerDiscriminator,omitempty"`
	SelectorProvenance     *BindingInspectionSelectorProvenance     `json:"selectorProvenance,omitempty"`
}

// BindingInspectionGVK describes one discovered served input kind.
type BindingInspectionGVK struct {
	Group   string `json:"group,omitempty"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// BindingInspectionContainerDiscriminator records the resolved container narrowing input.
type BindingInspectionContainerDiscriminator struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// BindingInspectionSelectorProvenance records how the effective selector set was assembled.
type BindingInspectionSelectorProvenance struct {
	SelectorSource       string   `json:"selectorSource,omitempty"`
	PoolDerivedSelectors []string `json:"poolDerivedSelectors,omitempty"`
	WorkloadSelectors    []string `json:"workloadSelectors,omitempty"`
	ContainerSelector    string   `json:"containerSelector,omitempty"`
	SafetySelectors      []string `json:"safetySelectors,omitempty"`
}

// BindingInspectionDesiredState captures the deterministic output kleym would render.
type BindingInspectionDesiredState struct {
	ClusterSPIFFEIDName string                               `json:"clusterSPIFFEIDName,omitempty"`
	SPIFFEID            string                               `json:"spiffeID,omitempty"`
	PodSelector         map[string]any                       `json:"podSelector,omitempty"`
	WorkloadSelectors   []string                             `json:"workloadSelectors,omitempty"`
	SelectorProvenance  *BindingInspectionSelectorProvenance `json:"selectorProvenance,omitempty"`
	Hint                string                               `json:"hint,omitempty"`
	Fallback            *bool                                `json:"fallback,omitempty"`
}

// BindingInspectionObservedState captures current cluster state relevant to the binding.
type BindingInspectionObservedState struct {
	ManagedClusterSPIFFEIDs []BindingInspectionManagedClusterSPIFFEID `json:"managedClusterSPIFFEIDs,omitempty"`
	Drift                   []BindingInspectionDriftEntry             `json:"drift,omitempty"`
	EligibleWorkloads       []BindingInspectionEligibleWorkload       `json:"eligibleWorkloads,omitempty"`
}

// BindingInspectionManagedClusterSPIFFEID summarizes one managed rendered object.
type BindingInspectionManagedClusterSPIFFEID struct {
	Name              string             `json:"name,omitempty"`
	SPIFFEID          string             `json:"spiffeID,omitempty"`
	PodSelector       map[string]any     `json:"podSelector,omitempty"`
	WorkloadSelectors []string           `json:"workloadSelectors,omitempty"`
	Hint              string             `json:"hint,omitempty"`
	Fallback          *bool              `json:"fallback,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
}

// BindingInspectionDriftEntry summarizes one desired-versus-observed mismatch.
type BindingInspectionDriftEntry struct {
	Field    string `json:"field,omitempty"`
	Desired  string `json:"desired,omitempty"`
	Observed string `json:"observed,omitempty"`
}

// BindingInspectionEligibleWorkload records one currently matching pod or container.
type BindingInspectionEligibleWorkload struct {
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
}

// BindingInspectionFinding is the stable machine-readable issue shape from docs/spec/cli.md.
type BindingInspectionFinding struct {
	ID           string                           `json:"id"`
	Severity     BindingInspectionFindingSeverity `json:"severity"`
	Reason       string                           `json:"reason"`
	Message      string                           `json:"message"`
	AffectedRefs []BindingInspectionTargetRef     `json:"affectedRefs"`
}

// BindingInspectionCapabilities records which inspection areas were complete.
type BindingInspectionCapabilities struct {
	Binding          BindingInspectionCapability `json:"binding,omitempty"`
	GAIEResources    BindingInspectionCapability `json:"gaieResources,omitempty"`
	ClusterSPIFFEIDs BindingInspectionCapability `json:"clusterspiffeids,omitempty"`
	PeerBindings     BindingInspectionCapability `json:"peerBindings,omitempty"`
	Pods             BindingInspectionCapability `json:"pods,omitempty"`
}

// NewBindingInspectionReport returns the stable report scaffold from docs/spec/cli.md.
func NewBindingInspectionReport() BindingInspectionReport {
	return normalizeBindingInspectionReport(BindingInspectionReport{})
}

func normalizeBindingInspectionReport(report BindingInspectionReport) BindingInspectionReport {
	if report.SchemaVersion == "" {
		report.SchemaVersion = BindingInspectionReportSchemaVersion
	}
	if report.Kind == "" {
		report.Kind = BindingInspectionReportKind
	}
	if report.Findings == nil {
		report.Findings = []BindingInspectionFinding{}
	}
	for i := range report.Findings {
		if report.Findings[i].AffectedRefs == nil {
			report.Findings[i].AffectedRefs = []BindingInspectionTargetRef{}
		}
	}
	return report
}

// writeBindingInspectionReportJSON writes newline-terminated compact JSON for the stable CLI contract.
func writeBindingInspectionReportJSON(w io.Writer, report BindingInspectionReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(normalizeBindingInspectionReport(report))
}

// writeBindingInspectionReportText writes deterministic human-oriented inspection output.
func writeBindingInspectionReportText(w io.Writer, report BindingInspectionReport) error {
	report = normalizeBindingInspectionReport(report)

	var builder strings.Builder
	appendTextLine(&builder, "BindingInspectionReport")
	appendTextLine(&builder, "GeneratedAt: %s", textString(report.GeneratedAt))
	appendTextLine(&builder, "")
	appendTextLine(&builder, "Binding:")
	appendTextLine(&builder, "  Name: %s", textNamespacedName(report.BindingRef.Namespace, report.BindingRef.Name))
	appendTextLine(&builder, "  Generation: %d", report.BindingRef.Generation)
	appendTextLine(&builder, "  Mode: %s", textString(report.BindingRef.Mode))
	appendTextLine(&builder, "  PoolRef: %s", textTargetRef(report.BindingRef.PoolRef))
	appendTextLine(&builder, "  ObjectiveRef: %s", textTargetRef(report.BindingRef.ObjectiveRef))
	appendTextLine(&builder, "  Conditions: %d", len(report.BindingRef.Conditions))

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Desired:")
	appendTextLine(&builder, "  ClusterSPIFFEID: %s", textString(report.Desired.ClusterSPIFFEIDName))
	appendTextLine(&builder, "  SPIFFE ID: %s", textString(report.Desired.SPIFFEID))
	appendTextLine(&builder, "  Pod selector: %s", textStableValue(report.Desired.PodSelector))
	appendTextStringList(&builder, "  Workload selectors", report.Desired.WorkloadSelectors)
	appendTextLine(&builder, "  Hint: %s", textString(report.Desired.Hint))
	appendTextLine(&builder, "  Fallback: %s", textBool(report.Desired.Fallback))

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Observed:")
	appendTextLine(&builder, "  Managed ClusterSPIFFEIDs: %d", len(report.Observed.ManagedClusterSPIFFEIDs))
	for _, managed := range report.Observed.ManagedClusterSPIFFEIDs {
		appendTextLine(&builder, "    - %s", textString(managed.Name))
		appendTextLine(&builder, "      SPIFFE ID: %s", textString(managed.SPIFFEID))
		appendTextLine(&builder, "      Pod selector: %s", textStableValue(managed.PodSelector))
		appendTextStringList(&builder, "      Workload selectors", managed.WorkloadSelectors)
		appendTextLine(&builder, "      Hint: %s", textString(managed.Hint))
		appendTextLine(&builder, "      Fallback: %s", textBool(managed.Fallback))
		appendTextLine(&builder, "      Conditions: %d", len(managed.Conditions))
	}
	appendTextLine(&builder, "  Drift: %d", len(report.Observed.Drift))
	for _, drift := range report.Observed.Drift {
		appendTextLine(&builder, "    - %s: desired=%s observed=%s", drift.Field, textString(drift.Desired), textString(drift.Observed))
	}
	appendTextLine(&builder, "  Eligible workloads: %d", len(report.Observed.EligibleWorkloads))
	for _, workload := range report.Observed.EligibleWorkloads {
		appendTextLine(&builder, "    - %s/%s container=%s", textString(workload.Namespace), textString(workload.Pod), textString(workload.Container))
	}

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Findings:")
	if len(report.Findings) == 0 {
		appendTextLine(&builder, "  none")
	} else {
		for _, finding := range report.Findings {
			appendTextLine(&builder, "  - %s %s (%s): %s", finding.Severity, finding.ID, finding.Reason, finding.Message)
			appendTextLine(&builder, "    AffectedRefs: %d", len(finding.AffectedRefs))
		}
	}

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Capabilities:")
	appendTextLine(&builder, "  Binding: %s", textString(string(report.Capabilities.Binding)))
	appendTextLine(&builder, "  GAIE resources: %s", textString(string(report.Capabilities.GAIEResources)))
	appendTextLine(&builder, "  ClusterSPIFFEIDs: %s", textString(string(report.Capabilities.ClusterSPIFFEIDs)))
	appendTextLine(&builder, "  Peer bindings: %s", textString(string(report.Capabilities.PeerBindings)))
	appendTextLine(&builder, "  Pods: %s", textString(string(report.Capabilities.Pods)))

	_, err := io.WriteString(w, builder.String())
	return err
}

// WriteBindingInspectionReport selects the binding inspection output format.
func WriteBindingInspectionReport(w io.Writer, output string, report BindingInspectionReport) error {
	switch output {
	case outputText:
		return writeBindingInspectionReportText(w, report)
	case outputJSON:
		return writeBindingInspectionReportJSON(w, report)
	default:
		return fmt.Errorf("invalid binding inspection output %q", output)
	}
}

func appendTextLine(builder *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(builder, format, args...)
	_ = builder.WriteByte('\n')
}

func appendTextStringList(builder *strings.Builder, label string, values []string) {
	appendTextLine(builder, "%s: %d", label, len(values))
	itemIndent := strings.Repeat(" ", leadingSpaces(label)+2)
	for _, value := range values {
		appendTextLine(builder, "%s- %s", itemIndent, value)
	}
}

func leadingSpaces(value string) int {
	for i, r := range value {
		if r != ' ' {
			return i
		}
	}
	return len(value)
}

func textString(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func textBool(value *bool) string {
	if value == nil {
		return "(none)"
	}
	if *value {
		return "true"
	}
	return "false"
}

func textNamespacedName(namespace string, name string) string {
	if namespace == "" {
		return textString(name)
	}
	if name == "" {
		return namespace + "/(none)"
	}
	return namespace + "/" + name
}

func textTargetRef(ref *BindingInspectionTargetRef) string {
	if ref == nil {
		return "(none)"
	}
	parts := make([]string, 0, 2)
	if ref.Kind != "" {
		kind := ref.Kind
		if ref.Version != "" {
			kind = ref.Version + "/" + kind
		}
		if ref.Group != "" {
			kind = ref.Group + "/" + kind
		}
		parts = append(parts, kind)
	}
	parts = append(parts, textNamespacedName(ref.Namespace, ref.Name))
	return strings.Join(parts, " ")
}

func textStableValue(value any) string {
	if value == nil {
		return "(none)"
	}
	text := stableValueString(value)
	if text == "" {
		return "(none)"
	}
	return text
}
