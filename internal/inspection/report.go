package inspection

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	outputText     = "text"
	outputJSON     = "json"
	outputYAML     = "yaml"
	outputMarkdown = "markdown"
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

// writeBindingInspectionReportYAML writes the normalized JSON report as equivalent YAML.
func writeBindingInspectionReportYAML(w io.Writer, report BindingInspectionReport) error {
	data, err := json.Marshal(normalizeBindingInspectionReport(report))
	if err != nil {
		return err
	}
	yamlData, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	if len(yamlData) == 0 || yamlData[len(yamlData)-1] != '\n' {
		yamlData = append(yamlData, '\n')
	}
	_, err = w.Write(yamlData)
	return err
}

// writeBindingInspectionReportMarkdown writes a PR-comment-friendly view backed by canonical JSON.
func writeBindingInspectionReportMarkdown(w io.Writer, report BindingInspectionReport) error {
	report = normalizeBindingInspectionReport(report)
	canonical, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	var builder strings.Builder
	appendMarkdownLine(&builder, "# BindingInspectionReport")
	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "## Summary")
	appendMarkdownTable(&builder, []string{"Field", "Value"}, [][]string{
		{"Status", inspectionTextStatus(report.Findings)},
		{"Findings", textCountOrNone(len(report.Findings))},
		{"Drift", textCountOrNone(len(report.Observed.Drift))},
		{"Eligible workloads", fmt.Sprintf("%d", len(report.Observed.EligibleWorkloads))},
		{"Inspection completeness", inspectionTextCompleteness(report.Capabilities)},
	})

	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "## Binding")
	appendMarkdownTable(&builder, []string{"Field", "Value"}, [][]string{
		{"Name", textNamespacedName(report.BindingRef.Namespace, report.BindingRef.Name)},
		{"Generation", fmt.Sprintf("%d", report.BindingRef.Generation)},
		{"Mode", textString(report.BindingRef.Mode)},
		{"PoolRef", textTargetRef(report.BindingRef.PoolRef)},
		{"ObjectiveRef", textTargetRef(report.BindingRef.ObjectiveRef)},
	})

	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "## Identity")
	appendMarkdownTable(&builder, []string{"Field", "Value"}, [][]string{
		{"ClusterSPIFFEID", textString(report.Desired.ClusterSPIFFEIDName)},
		{"SPIFFE ID", textString(report.Desired.SPIFFEID)},
		{"Hint", textString(report.Desired.Hint)},
		{"Fallback", textBool(report.Desired.Fallback)},
	})

	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "## Findings")
	if len(report.Findings) == 0 {
		appendMarkdownLine(&builder, "No findings.")
	} else {
		rows := make([][]string, 0, len(report.Findings))
		for _, finding := range report.Findings {
			rows = append(rows, []string{
				textString(finding.ID),
				textString(string(finding.Severity)),
				textString(finding.Reason),
				textString(finding.Message),
				markdownTargetRefs(finding.AffectedRefs),
			})
		}
		appendMarkdownTable(&builder, []string{"ID", "Severity", "Reason", "Message", "Affected refs"}, rows)
	}

	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "## Capabilities")
	appendMarkdownTable(&builder, []string{"Check", "Completeness"}, [][]string{
		{"Binding", textString(string(report.Capabilities.Binding))},
		{"GAIE resources", textString(string(report.Capabilities.GAIEResources))},
		{"ClusterSPIFFEIDs", textString(string(report.Capabilities.ClusterSPIFFEIDs))},
		{"Peer bindings", textString(string(report.Capabilities.PeerBindings))},
		{"Pods", textString(string(report.Capabilities.Pods))},
	})

	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "<details>")
	appendMarkdownLine(&builder, "<summary>Canonical JSON report</summary>")
	appendMarkdownLine(&builder, "")
	appendMarkdownLine(&builder, "```json")
	appendMarkdownLine(&builder, "%s", canonical)
	appendMarkdownLine(&builder, "```")
	appendMarkdownLine(&builder, "</details>")

	_, err = io.WriteString(w, builder.String())
	return err
}

// writeBindingInspectionReportText writes deterministic human-oriented inspection output.
func writeBindingInspectionReportText(w io.Writer, report BindingInspectionReport) error {
	report = normalizeBindingInspectionReport(report)

	var builder strings.Builder
	appendTextLine(&builder, "BindingInspectionReport")
	appendTextLine(&builder, "GeneratedAt: %s", textString(report.GeneratedAt))
	appendTextLine(&builder, "")
	appendTextLine(&builder, "Summary:")
	appendTextLine(&builder, "  Status: %s", inspectionTextStatus(report.Findings))
	appendTextLine(&builder, "  Findings: %s", textCountOrNone(len(report.Findings)))
	appendTextLine(&builder, "  Drift: %s", textCountOrNone(len(report.Observed.Drift)))
	appendTextLine(&builder, "  Eligible workloads: %d", len(report.Observed.EligibleWorkloads))
	appendTextLine(&builder, "  Inspection completeness: %s", inspectionTextCompleteness(report.Capabilities))
	partialChecks := inspectionTextCapabilityNames(report.Capabilities, BindingInspectionCapabilityPartial)
	if len(partialChecks) > 0 {
		appendTextLine(&builder, "  Partial checks:")
		for _, check := range partialChecks {
			appendTextLine(&builder, "    - %s", check)
		}
	}

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Binding:")
	appendTextLine(&builder, "  Name: %s", textNamespacedName(report.BindingRef.Namespace, report.BindingRef.Name))
	appendTextLine(&builder, "  Generation: %d", report.BindingRef.Generation)
	appendTextLine(&builder, "  Mode: %s", textString(report.BindingRef.Mode))
	appendTextLine(&builder, "  PoolRef: %s", textTargetRef(report.BindingRef.PoolRef))
	appendTextLine(&builder, "  ObjectiveRef: %s", textTargetRef(report.BindingRef.ObjectiveRef))
	appendTextConditions(&builder, "  ", report.BindingRef.Conditions)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Identity:")
	appendTextLine(&builder, "  ClusterSPIFFEID: %s", textString(report.Desired.ClusterSPIFFEIDName))
	appendTextLine(&builder, "  SPIFFE ID: %s", textString(report.Desired.SPIFFEID))
	appendTextLine(&builder, "  Hint: %s", textString(report.Desired.Hint))
	appendTextLine(&builder, "  Fallback: %s", textBool(report.Desired.Fallback))

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Selectors:")
	appendTextLine(&builder, "  Pod selector: %s", textStableValue(report.Desired.PodSelector))
	appendTextStringList(&builder, "  Workload selectors", report.Desired.WorkloadSelectors)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Observed:")
	appendTextLine(&builder, "  Managed ClusterSPIFFEIDs: %d", len(report.Observed.ManagedClusterSPIFFEIDs))
	for _, managed := range report.Observed.ManagedClusterSPIFFEIDs {
		appendTextLine(&builder, "    - %s", textString(managed.Name))
		if len(report.Observed.Drift) == 0 {
			appendTextLine(&builder, "      Status: matches desired")
		} else {
			appendTextLine(&builder, "      Status: drift detected")
			appendTextDriftEntries(&builder, "      ", report.Observed.Drift)
		}
		appendTextConditions(&builder, "      ", managed.Conditions)
	}
	if len(report.Observed.ManagedClusterSPIFFEIDs) == 0 && len(report.Observed.Drift) > 0 {
		appendTextDriftEntries(&builder, "  ", report.Observed.Drift)
	}
	appendTextEligibleWorkloads(&builder, report.Observed.EligibleWorkloads)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Findings:")
	if len(report.Findings) == 0 {
		appendTextLine(&builder, "  none")
	} else {
		for _, finding := range report.Findings {
			appendTextLine(&builder, "  - Severity: %s", textString(string(finding.Severity)))
			appendTextLine(&builder, "    Reason: %s", textString(finding.Reason))
			appendTextLine(&builder, "    Message: %s", textString(finding.Message))
			appendTextLine(&builder, "    AffectedRefs:")
			if len(finding.AffectedRefs) == 0 {
				appendTextLine(&builder, "      none")
			}
			for i := range finding.AffectedRefs {
				appendTextLine(&builder, "      - %s", textTargetRef(&finding.AffectedRefs[i]))
			}
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
	case outputYAML:
		return writeBindingInspectionReportYAML(w, report)
	case outputMarkdown:
		return writeBindingInspectionReportMarkdown(w, report)
	default:
		return fmt.Errorf("invalid binding inspection output %q", output)
	}
}

func appendMarkdownLine(builder *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(builder, format, args...)
	_ = builder.WriteByte('\n')
}

func appendMarkdownTable(builder *strings.Builder, headers []string, rows [][]string) {
	appendMarkdownLine(builder, "| %s |", strings.Join(markdownEscapedCells(headers), " | "))
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	appendMarkdownLine(builder, "| %s |", strings.Join(separators, " | "))
	for _, row := range rows {
		appendMarkdownLine(builder, "| %s |", strings.Join(markdownEscapedCells(row), " | "))
	}
}

func markdownEscapedCells(values []string) []string {
	escaped := make([]string, len(values))
	for i, value := range values {
		escaped[i] = markdownEscapeCell(value)
	}
	return escaped
}

func markdownEscapeCell(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}

func markdownTargetRefs(refs []BindingInspectionTargetRef) string {
	if len(refs) == 0 {
		return "none"
	}
	values := make([]string, 0, len(refs))
	for i := range refs {
		values = append(values, textTargetRef(&refs[i]))
	}
	return strings.Join(values, "<br>")
}

func appendTextLine(builder *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(builder, format, args...)
	_ = builder.WriteByte('\n')
}

func appendTextStringList(builder *strings.Builder, label string, values []string) {
	appendTextLine(builder, "%s:", label)
	itemIndent := strings.Repeat(" ", leadingSpaces(label)+2)
	if len(values) == 0 {
		appendTextLine(builder, "%snone", itemIndent)
		return
	}
	for _, value := range values {
		appendTextLine(builder, "%s- %s", itemIndent, value)
	}
}

func appendTextConditions(builder *strings.Builder, indent string, conditions []metav1.Condition) {
	appendTextLine(builder, "%sConditions:", indent)
	if len(conditions) == 0 {
		appendTextLine(builder, "%s  none", indent)
		return
	}
	for _, condition := range conditions {
		appendTextLine(builder, "%s  %s=%s", indent, condition.Type, condition.Status)
		if !textConditionNeedsDetail(condition) {
			continue
		}
		appendTextLine(builder, "%s    Reason: %s", indent, textString(condition.Reason))
		appendTextLine(builder, "%s    Message: %s", indent, textString(condition.Message))
	}
}

func appendTextDriftEntries(builder *strings.Builder, indent string, entries []BindingInspectionDriftEntry) {
	appendTextLine(builder, "%sDrift:", indent)
	if len(entries) == 0 {
		appendTextLine(builder, "%s  none", indent)
		return
	}
	for _, drift := range entries {
		appendTextLine(builder, "%s  - Field: %s", indent, textDriftField(drift.Field))
		appendTextLine(builder, "%s    Desired: %s", indent, textString(drift.Desired))
		appendTextLine(builder, "%s    Observed: %s", indent, textString(drift.Observed))
	}
}

func appendTextEligibleWorkloads(builder *strings.Builder, workloads []BindingInspectionEligibleWorkload) {
	appendTextLine(builder, "  Eligible workloads:")
	if len(workloads) == 0 {
		appendTextLine(builder, "    none")
		return
	}
	for _, workload := range workloads {
		line := textNamespacedName(workload.Namespace, workload.Pod)
		if workload.Container != "" {
			line += " container=" + workload.Container
		}
		appendTextLine(builder, "    - %s", line)
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

func inspectionTextStatus(findings []BindingInspectionFinding) string {
	if HasErrorSeverityFinding(findings) {
		return "Error"
	}
	if HasWarningSeverityFinding(findings) {
		return "Warning"
	}
	return "OK"
}

func inspectionTextCompleteness(capabilities BindingInspectionCapabilities) string {
	checks := inspectionTextCapabilityChecks(capabilities)
	hasCapability := false
	hasSkipped := false
	hasUnknown := false
	for _, check := range checks {
		if check.value == "" {
			hasUnknown = true
			continue
		}
		hasCapability = true
		switch check.value {
		case BindingInspectionCapabilityPartial:
			return string(BindingInspectionCapabilityPartial)
		case BindingInspectionCapabilitySkipped:
			hasSkipped = true
		case BindingInspectionCapabilityUnknown:
			hasUnknown = true
		case BindingInspectionCapabilityFull:
		default:
			hasUnknown = true
		}
	}
	if hasSkipped {
		return string(BindingInspectionCapabilitySkipped)
	}
	if hasUnknown || !hasCapability {
		return string(BindingInspectionCapabilityUnknown)
	}
	return string(BindingInspectionCapabilityFull)
}

func inspectionTextCapabilityNames(
	capabilities BindingInspectionCapabilities,
	want BindingInspectionCapability,
) []string {
	checks := inspectionTextCapabilityChecks(capabilities)
	names := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.value == want {
			names = append(names, check.name)
		}
	}
	return names
}

type inspectionTextCapabilityCheck struct {
	name  string
	value BindingInspectionCapability
}

func inspectionTextCapabilityChecks(capabilities BindingInspectionCapabilities) []inspectionTextCapabilityCheck {
	return []inspectionTextCapabilityCheck{
		{name: "Binding", value: capabilities.Binding},
		{name: "GAIE resources", value: capabilities.GAIEResources},
		{name: "ClusterSPIFFEIDs", value: capabilities.ClusterSPIFFEIDs},
		{name: "Peer bindings", value: capabilities.PeerBindings},
		{name: "Pods", value: capabilities.Pods},
	}
}

func textConditionNeedsDetail(condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionUnknown {
		return true
	}
	switch condition.Type {
	case "Ready", "Resolved", "Rendered":
		return condition.Status != metav1.ConditionTrue
	case "Conflict", "InvalidRef", "UnsafeSelector", "RenderFailure":
		return condition.Status == metav1.ConditionTrue
	default:
		return condition.Status != metav1.ConditionTrue
	}
}

func textCountOrNone(count int) string {
	if count == 0 {
		return "none"
	}
	return fmt.Sprintf("%d", count)
}

func textDriftField(field string) string {
	switch field {
	case "spec.spiffeIDTemplate":
		return "spiffeID"
	case "spec.podSelector":
		return "podSelector"
	case "spec.workloadSelectorTemplates":
		return "workloadSelectorTemplates"
	case "spec.hint":
		return "hint"
	case "spec.fallback":
		return "fallback"
	default:
		return field
	}
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
