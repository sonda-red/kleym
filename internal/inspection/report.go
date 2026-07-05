package inspection

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	outputText = "text"
	outputJSON = "json"
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

// BindingInspectionCapability is the internal completeness enum used while building findings and text output.
type BindingInspectionCapability string

// BindingInspectionReport captures the JSON contract for `kleym inspect binding -o json`.
type BindingInspectionReport struct {
	SchemaVersion           string                                   `json:"schemaVersion"`
	Kind                    string                                   `json:"kind"`
	GeneratedAt             string                                   `json:"generatedAt"`
	IdentityConfig          BindingInspectionIdentityConfig          `json:"identityConfig,omitempty"`
	BindingRef              BindingInspectionBindingRef              `json:"bindingRef"`
	Resolved                BindingInspectionResolvedInput           `json:"resolvedInput"`
	RenderedIdentity        BindingInspectionRenderedIdentity        `json:"renderedIdentity"`
	RenderedClusterSPIFFEID BindingInspectionRenderedClusterSPIFFEID `json:"renderedClusterSPIFFEID"`
	MatchedPods             []BindingInspectionMatchedPod            `json:"matchedPods"`
	Findings                []BindingInspectionFinding               `json:"findings"`

	Capabilities BindingInspectionCapabilities `json:"-"`
	ExitCode     *int                          `json:"-"`
}

// BindingInspectionIdentityConfig records which identity configuration inspection used.
type BindingInspectionIdentityConfig struct {
	TrustDomain                    string `json:"trustDomain,omitempty"`
	TrustDomainSource              string `json:"trustDomainSource,omitempty"`
	ClusterSPIFFEIDClassName       string `json:"clusterSPIFFEIDClassName,omitempty"`
	ClusterSPIFFEIDClassNameSource string `json:"clusterSPIFFEIDClassNameSource,omitempty"`
}

// BindingInspectionBindingRef identifies the binding being inspected.
type BindingInspectionBindingRef struct {
	Namespace  string                      `json:"namespace,omitempty"`
	Name       string                      `json:"name,omitempty"`
	Generation int64                       `json:"generation,omitempty"`
	PoolRef    *BindingInspectionTargetRef `json:"poolRef,omitempty"`
	Conditions []metav1.Condition          `json:"conditions,omitempty"`
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
	PoolRef            *BindingInspectionTargetRef          `json:"poolRef,omitempty"`
	ServedGVKs         []BindingInspectionGVK               `json:"servedGVKs,omitempty"`
	PoolSelector       map[string]any                       `json:"poolSelector,omitempty"`
	SelectorProvenance *BindingInspectionSelectorProvenance `json:"selectorProvenance,omitempty"`
}

// BindingInspectionGVK describes one discovered served input kind.
type BindingInspectionGVK struct {
	Group   string `json:"group,omitempty"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// BindingInspectionSelectorProvenance records how the effective selector set was assembled.
type BindingInspectionSelectorProvenance struct {
	PoolDerivedSelectors []string `json:"poolDerivedSelectors,omitempty"`
	SafetySelectors      []string `json:"safetySelectors,omitempty"`
}

// BindingInspectionRenderedIdentity captures the identity fields rendered from the binding.
type BindingInspectionRenderedIdentity struct {
	SPIFFEID           string                               `json:"spiffeID,omitempty"`
	PodSelector        map[string]any                       `json:"podSelector,omitempty"`
	WorkloadSelectors  []string                             `json:"workloadSelectors,omitempty"`
	SelectorProvenance *BindingInspectionSelectorProvenance `json:"selectorProvenance,omitempty"`
}

// BindingInspectionRenderedClusterSPIFFEID captures the managed object fields Kleym renders.
type BindingInspectionRenderedClusterSPIFFEID struct {
	Name              string         `json:"name,omitempty"`
	SPIFFEID          string         `json:"spiffeID,omitempty"`
	PodSelector       map[string]any `json:"podSelector,omitempty"`
	WorkloadSelectors []string       `json:"workloadSelectors,omitempty"`
	Hint              string         `json:"hint,omitempty"`
	ClassName         string         `json:"className,omitempty"`
	Fallback          *bool          `json:"fallback,omitempty"`
}

// BindingInspectionMatchedPod records one currently matching pod or container.
type BindingInspectionMatchedPod struct {
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

// BindingInspectionCapabilities records internal inspection completeness.
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
	if report.MatchedPods == nil {
		report.MatchedPods = []BindingInspectionMatchedPod{}
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
	appendTextLine(&builder, "Binding: %s", textNamespacedName(report.BindingRef.Namespace, report.BindingRef.Name))
	appendTextLine(&builder, "Source: %s", textShortTargetRef(firstTargetRef(report.Resolved.PoolRef, report.BindingRef.PoolRef), "InferencePool"))

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Identity:")
	appendTextIdentity(&builder, report.RenderedIdentity)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "ClusterSPIFFEID:")
	appendTextClusterSPIFFEID(&builder, report.RenderedClusterSPIFFEID)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Conditions:")
	appendTextConditionList(&builder, "  ", report.BindingRef.Conditions)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Matched pods:")
	appendTextMatchedPods(&builder, report)

	appendTextLine(&builder, "")
	appendTextFindings(&builder, report.Findings)

	appendTextLine(&builder, "Exit code: %d", textExitCode(report))

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

func appendTextIdentity(builder *strings.Builder, identity BindingInspectionRenderedIdentity) {
	if identity.SPIFFEID == "" {
		appendTextLine(builder, "  not rendered")
		return
	}
	appendTextLine(builder, "  SPIFFE ID: %s", identity.SPIFFEID)
	appendTextSelectorSummary(builder, identity.WorkloadSelectors)
}

func appendTextClusterSPIFFEID(builder *strings.Builder, clusterspiffeid BindingInspectionRenderedClusterSPIFFEID) {
	if clusterspiffeid.Name == "" {
		appendTextLine(builder, "  not rendered")
		return
	}
	appendTextLine(builder, "  Name: %s", clusterspiffeid.Name)
	appendTextLine(builder, "  ClassName: %s", textString(clusterspiffeid.ClassName))
	appendTextLine(builder, "  Hint: %s", textString(clusterspiffeid.Hint))
	if clusterspiffeid.Fallback != nil && *clusterspiffeid.Fallback {
		appendTextLine(builder, "  Fallback: true")
	}
}

func appendTextSelectorSummary(builder *strings.Builder, selectors []string) {
	selection := workloadSelectionFromSelectors(selectors)
	appendTextLine(builder, "  Selectors:")
	if selection.Namespace != "" {
		appendTextLine(builder, "    namespace: %s", selection.Namespace)
	}
	if selection.ServiceAccount != "" {
		appendTextLine(builder, "    serviceAccount: %s", selection.ServiceAccount)
	}
	appendTextPodLabels(builder, selection.PodLabels)
	if selection.ContainerSelectorType == "name" {
		appendTextLine(builder, "    container: %s", selection.ContainerValue)
	}
	if len(selection.UnsupportedSelectors) > 0 {
		appendTextStringList(builder, "    unsupported", selection.UnsupportedSelectors)
	}
}

func appendTextPodLabels(builder *strings.Builder, labels map[string]string) {
	if len(labels) == 0 {
		return
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+labels[key])
	}
	appendTextLine(builder, "    podLabels: %s", strings.Join(values, ", "))
}

func appendTextConditionList(builder *strings.Builder, indent string, conditions []metav1.Condition) {
	if len(conditions) == 0 {
		appendTextLine(builder, "%snone", indent)
		return
	}
	for _, condition := range conditions {
		appendTextLine(builder, "%s%s=%s", indent, condition.Type, condition.Status)
		if !textConditionNeedsDetail(condition) {
			continue
		}
		appendTextLine(builder, "%s    Reason: %s", indent, textString(condition.Reason))
		appendTextLine(builder, "%s    Message: %s", indent, textString(condition.Message))
	}
}

func appendTextMatchedPods(builder *strings.Builder, report BindingInspectionReport) {
	if report.RenderedIdentity.SPIFFEID == "" {
		appendTextLine(builder, "  skipped")
		return
	}
	if report.Capabilities.Pods == BindingInspectionCapabilityPartial ||
		report.Capabilities.Pods == BindingInspectionCapabilityUnknown {
		appendTextLine(builder, "  unknown")
		return
	}
	if report.Capabilities.Pods == BindingInspectionCapabilitySkipped {
		appendTextLine(builder, "  skipped")
		return
	}
	if len(report.MatchedPods) == 0 {
		appendTextLine(builder, "  none")
		return
	}
	for _, workload := range report.MatchedPods {
		line := textNamespacedName(workload.Namespace, workload.Pod)
		if workload.Container != "" {
			line += " container=" + workload.Container
		}
		appendTextLine(builder, "  %s", line)
	}
}

func appendTextFindings(builder *strings.Builder, findings []BindingInspectionFinding) {
	if len(findings) == 0 {
		appendTextLine(builder, "Findings: none")
		return
	}
	appendTextLine(builder, "Findings:")
	for _, finding := range findings {
		appendTextLine(builder, "  %s %s", textFindingSeverity(finding.Severity), textString(finding.Reason))
		if finding.Message != "" {
			appendTextLine(builder, "    %s", finding.Message)
		}
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

func textExitCode(report BindingInspectionReport) int {
	if report.ExitCode != nil {
		return *report.ExitCode
	}
	if HasErrorSeverityFinding(report.Findings) {
		return 2
	}
	return 0
}

func textFindingSeverity(severity BindingInspectionFindingSeverity) string {
	switch severity {
	case BindingInspectionFindingSeverityError:
		return "Error"
	case BindingInspectionFindingSeverityWarning:
		return "Warning"
	case BindingInspectionFindingSeverityInfo:
		return "Info"
	default:
		return textString(string(severity))
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

func textString(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
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

func textShortTargetRef(ref *BindingInspectionTargetRef, fallbackKind string) string {
	if ref == nil {
		return "(missing)"
	}
	kind := ref.Kind
	if kind == "" {
		kind = fallbackKind
	}
	return strings.TrimSpace(kind + " " + textNamespacedName(ref.Namespace, ref.Name))
}

func firstTargetRef(refs ...*BindingInspectionTargetRef) *BindingInspectionTargetRef {
	for _, ref := range refs {
		if ref != nil {
			return ref
		}
	}
	return nil
}
