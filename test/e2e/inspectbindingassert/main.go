package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sonda-red/kleym/internal/inspection"
)

const (
	referenceNamespace       = "kleym-reference-inference"
	referenceBinding         = "binding"
	referenceSPIFFEID        = "spiffe://kleym.sonda.red/ns/kleym-reference-inference/sa/reference-inference/inference/pool/reference-pool"
	referenceClusterSPIFFEID = "kleym-kleym-reference-inference-binding-pool-e1a1f353"
)

func main() {
	if len(os.Args) != 3 {
		failf("usage: inspectbindingassert <report.json> <success|not-found>")
	}

	report, err := readReport(os.Args[1])
	if err != nil {
		failf("%v", err)
	}

	switch os.Args[2] {
	case "success":
		assertSuccess(report)
	case "not-found":
		assertNotFound(report)
	default:
		failf("unknown assertion mode %q", os.Args[2])
	}
}

// readReport decodes the CLI JSON report so Chainsaw can assert stable fields.
func readReport(path string) (inspection.BindingInspectionReport, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return inspection.BindingInspectionReport{}, fmt.Errorf("read report: %w", err)
	}

	var report inspection.BindingInspectionReport
	if err := json.Unmarshal(content, &report); err != nil {
		return inspection.BindingInspectionReport{}, fmt.Errorf("decode report JSON: %w", err)
	}
	return report, nil
}

// assertSuccess verifies live inspection saw the reconciled binding and workload.
func assertSuccess(report inspection.BindingInspectionReport) {
	assertCommonShape(report)
	assertEqual("bindingRef.namespace", report.BindingRef.Namespace, referenceNamespace)
	assertEqual("bindingRef.name", report.BindingRef.Name, referenceBinding)
	assertEqual("renderedClusterSPIFFEID.name", report.RenderedClusterSPIFFEID.Name, referenceClusterSPIFFEID)
	assertEqual("renderedIdentity.spiffeID", report.RenderedIdentity.SPIFFEID, referenceSPIFFEID)

	if len(report.Findings) != 0 {
		failf("findings = %d, want 0: %#v", len(report.Findings), report.Findings)
	}
	assertMatchedPod(report)
}

// assertNotFound verifies the CLI still emits machine-readable JSON for a missing binding.
func assertNotFound(report inspection.BindingInspectionReport) {
	assertCommonShape(report)
	assertEqual("bindingRef.namespace", report.BindingRef.Namespace, referenceNamespace)

	for _, finding := range report.Findings {
		if finding.ID == "binding-not-found" && finding.Severity == inspection.BindingInspectionFindingSeverityError {
			return
		}
	}
	failf("missing error binding-not-found finding: %#v", report.Findings)
}

// assertCommonShape covers report fields every JSON inspection result must include.
func assertCommonShape(report inspection.BindingInspectionReport) {
	assertEqual("schemaVersion", report.SchemaVersion, inspection.BindingInspectionReportSchemaVersion)
	assertEqual("kind", report.Kind, inspection.BindingInspectionReportKind)
	if report.GeneratedAt == "" {
		failf("generatedAt is empty")
	}
}

// assertMatchedPod proves live pod visibility contributed to the inspection result.
func assertMatchedPod(report inspection.BindingInspectionReport) {
	for _, workload := range report.MatchedPods {
		if workload.Namespace == referenceNamespace && workload.Pod != "" && workload.Container == "" {
			return
		}
	}
	failf("missing matched pod-level workload: %#v", report.MatchedPods)
}

func assertEqual(field string, got string, want string) {
	if got != want {
		failf("%s = %q, want %q", field, got, want)
	}
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
