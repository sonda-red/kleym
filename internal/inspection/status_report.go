package inspection

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	// StatusReportSchemaVersion is the stable machine-readable status report version from docs/spec/cli.md.
	StatusReportSchemaVersion = "v1alpha1"
	// StatusReportKind is the stable report kind from docs/spec/cli.md.
	StatusReportKind = "KleymStatusReport"
)

const (
	StatusResultOK      StatusResult = "OK"
	StatusResultWarning StatusResult = "WARNING"
	StatusResultError   StatusResult = "ERROR"
	StatusResultUnknown StatusResult = "unknown"
)

// StatusResult is the stable result enum used by `kleym status`.
type StatusResult string

// StatusReport captures the JSON contract for `kleym status -o json`.
type StatusReport struct {
	SchemaVersion string                     `json:"schemaVersion"`
	Kind          string                     `json:"kind"`
	GeneratedAt   string                     `json:"generatedAt"`
	Status        StatusResult               `json:"status"`
	CLIVersion    string                     `json:"cliVersion,omitempty"`
	Components    StatusComponents           `json:"components"`
	Config        StatusConfig               `json:"config,omitempty"`
	Summary       StatusSummary              `json:"summary"`
	Findings      []BindingInspectionFinding `json:"findings"`

	ExitCode *int `json:"-"`
}

// StatusComponents records aggregate status for discoverable installation pieces.
type StatusComponents struct {
	Kleym     ComponentStatus `json:"kleym"`
	Operator  OperatorStatus  `json:"operator"`
	KleymCRDs KleymAPIStatus  `json:"kleymCRDs"`
	SPIRECRDs SPIRECRDStatus  `json:"spireCRDs"`
	GAIECRDs  GAIEStatus      `json:"gaieCRDs"`
}

// ComponentStatus describes one cluster-visible status component.
type ComponentStatus struct {
	Status  StatusResult `json:"status"`
	Message string       `json:"message,omitempty"`
}

// OperatorStatus describes the Kleym operator Deployment when it is visible.
type OperatorStatus struct {
	Status        StatusResult `json:"status"`
	Message       string       `json:"message,omitempty"`
	Deployment    string       `json:"deployment,omitempty"`
	ReadyReplicas int32        `json:"readyReplicas,omitempty"`
	Replicas      int32        `json:"replicas,omitempty"`
	Version       string       `json:"version,omitempty"`
}

// KleymAPIStatus records the Kleym API CRD versions used by status output.
type KleymAPIStatus struct {
	Status                   StatusResult `json:"status"`
	Message                  string       `json:"message,omitempty"`
	InferenceIdentityBinding string       `json:"inferenceIdentityBinding,omitempty"`
}

// SPIRECRDStatus records SPIRE CRD availability.
type SPIRECRDStatus struct {
	Status          StatusResult `json:"status"`
	Message         string       `json:"message,omitempty"`
	ClusterSPIFFEID string       `json:"clusterSPIFFEID,omitempty"`
}

// GAIEStatus records Gateway API Inference Extension CRD availability.
type GAIEStatus struct {
	Status        StatusResult `json:"status"`
	Message       string       `json:"message,omitempty"`
	InferencePool string       `json:"inferencePool,omitempty"`
}

// StatusConfig records visible operator identity configuration.
type StatusConfig struct {
	TrustDomain                   string `json:"trustDomain,omitempty"`
	ClusterSPIFFEIDClassName      string `json:"clusterSPIFFEIDClassName,omitempty"`
	ClusterSPIFFEIDClassNameKnown bool   `json:"clusterSPIFFEIDClassNameKnown,omitempty"`
}

// StatusSummary records aggregate binding counts.
type StatusSummary struct {
	Bindings BindingStatusSummary `json:"bindings"`
}

// BindingStatusSummary counts bindings by visible health state.
type BindingStatusSummary struct {
	OK         int                     `json:"ok"`
	Warning    int                     `json:"warning"`
	Error      int                     `json:"error"`
	Total      int                     `json:"total"`
	Conditions BindingConditionSummary `json:"conditions"`
}

// BindingConditionSummary counts true binding conditions by type.
type BindingConditionSummary struct {
	Ready          int `json:"ready"`
	InvalidRef     int `json:"invalidRef"`
	UnsafeSelector int `json:"unsafeSelector"`
	RenderFailure  int `json:"renderFailure"`
}

// NewStatusReport returns the stable report scaffold from docs/spec/cli.md.
func NewStatusReport() StatusReport {
	return normalizeStatusReport(StatusReport{})
}

func normalizeStatusReport(report StatusReport) StatusReport {
	if report.SchemaVersion == "" {
		report.SchemaVersion = StatusReportSchemaVersion
	}
	if report.Kind == "" {
		report.Kind = StatusReportKind
	}
	if report.Status == "" {
		report.Status = StatusResultUnknown
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

// WriteStatusReport selects the cluster status output format.
func WriteStatusReport(w io.Writer, output string, report StatusReport) error {
	switch output {
	case outputText:
		return writeStatusReportText(w, report)
	case outputJSON:
		return writeStatusReportJSON(w, report)
	default:
		return fmt.Errorf("invalid status output %q", output)
	}
}

func writeStatusReportJSON(w io.Writer, report StatusReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(normalizeStatusReport(report))
}

func writeStatusReportText(w io.Writer, report StatusReport) error {
	report = normalizeStatusReport(report)

	var builder strings.Builder
	appendTextLine(&builder, "Kleym")
	appendTextLine(&builder, "  CLI: %s", textString(report.CLIVersion))
	appendTextLine(&builder, "  Operator: %s", availabilityStatus(report.Components.Operator.Status))
	appendTextLine(&builder, "    Deployment: %s", textString(report.Components.Operator.Deployment))
	appendTextLine(&builder, "    Ready: %d/%d", report.Components.Operator.ReadyReplicas, report.Components.Operator.Replicas)
	appendTextLine(&builder, "    Version: %s", textString(report.Components.Operator.Version))
	appendTextLine(&builder, "  Config:")
	appendTextLine(&builder, "    trustDomain: %s", textConfigValue(report.Config.TrustDomain))
	appendTextLine(&builder, "    clusterSPIFFEIDClass: %s", textClassName(report.Config.ClusterSPIFFEIDClassName, report.Config.ClusterSPIFFEIDClassNameKnown))
	appendTextLine(&builder, "  API:")
	appendTextLine(&builder, "    InferenceIdentityBinding: %s", textString(report.Components.KleymCRDs.InferenceIdentityBinding))

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Bindings")
	appendTextLine(&builder, "  Total: %d", report.Summary.Bindings.Total)
	appendTextLine(&builder, "  Conditions:")
	appendTextLine(&builder, "    Ready: %d", report.Summary.Bindings.Conditions.Ready)
	appendTextLine(&builder, "    InvalidRef: %d", report.Summary.Bindings.Conditions.InvalidRef)
	appendTextLine(&builder, "    UnsafeSelector: %d", report.Summary.Bindings.Conditions.UnsafeSelector)
	appendTextLine(&builder, "    RenderFailure: %d", report.Summary.Bindings.Conditions.RenderFailure)

	appendTextLine(&builder, "")
	appendTextLine(&builder, "Dependencies")
	appendTextLine(&builder, "  GAIE: %s", availabilityStatus(report.Components.GAIECRDs.Status))
	appendTextLine(&builder, "    InferencePool: %s", textString(report.Components.GAIECRDs.InferencePool))
	appendTextLine(&builder, "  SPIRE: %s", availabilityStatus(report.Components.SPIRECRDs.Status))
	appendTextLine(&builder, "    ClusterSPIFFEID: %s", textString(report.Components.SPIRECRDs.ClusterSPIFFEID))

	if len(report.Findings) > 0 {
		appendTextLine(&builder, "")
		appendTextFindings(&builder, report.Findings)
	}

	_, err := io.WriteString(w, builder.String())
	return err
}

func availabilityStatus(status StatusResult) string {
	switch status {
	case "":
		return "unknown"
	case StatusResultOK:
		return "Available"
	case StatusResultWarning:
		return "Warning"
	case StatusResultError:
		return "Unavailable"
	default:
		return string(status)
	}
}

func textConfigValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func textClassName(className string, known bool) string {
	if !known {
		return "unknown"
	}
	if className == "" {
		return "classless"
	}
	return className
}
