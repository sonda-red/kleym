package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sonda-red/kleym/internal/inspection"
)

const (
	referenceNamespace             = "kleym-reference-inference"
	referenceBinding               = "binding"
	referenceSPIFFEID              = "spiffe://kleym.sonda.red/ns/kleym-reference-inference/objective/reference-objective"
	referenceClusterSPIFFEID       = "kleym-kleym-reference-inference-binding-objective-3b2d9110"
	referenceEligibleContainerName = "model-server"
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
	assertEqual("bindingRef.mode", report.BindingRef.Mode, "PerObjective")
	assertEqual("desired.clusterSPIFFEIDName", report.Desired.ClusterSPIFFEIDName, referenceClusterSPIFFEID)
	assertEqual("desired.spiffeID", report.Desired.SPIFFEID, referenceSPIFFEID)
	assertEqual("capabilities.clusterspiffeids", string(report.Capabilities.ClusterSPIFFEIDs), "full")
	assertEqual("capabilities.pods", string(report.Capabilities.Pods), "full")

	if len(report.Findings) != 0 {
		failf("findings = %d, want 0: %#v", len(report.Findings), report.Findings)
	}
	if len(report.Observed.Drift) != 0 {
		failf("observed.drift = %d, want 0: %#v", len(report.Observed.Drift), report.Observed.Drift)
	}
	if len(report.Observed.ManagedClusterSPIFFEIDs) != 1 {
		failf("managed ClusterSPIFFEIDs = %d, want 1", len(report.Observed.ManagedClusterSPIFFEIDs))
	}
	assertEqual("observed.managedClusterSPIFFEIDs[0].name", report.Observed.ManagedClusterSPIFFEIDs[0].Name, referenceClusterSPIFFEID)
	assertEqual("observed.managedClusterSPIFFEIDs[0].spiffeID", report.Observed.ManagedClusterSPIFFEIDs[0].SPIFFEID, referenceSPIFFEID)
	assertEligibleContainer(report)
}

// assertNotFound verifies the CLI still emits machine-readable JSON for a missing binding.
func assertNotFound(report inspection.BindingInspectionReport) {
	assertCommonShape(report)
	assertEqual("bindingRef.namespace", report.BindingRef.Namespace, referenceNamespace)
	assertEqual("capabilities.binding", string(report.Capabilities.Binding), "full")

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

// assertEligibleContainer proves live pod visibility contributed to the inspection result.
func assertEligibleContainer(report inspection.BindingInspectionReport) {
	for _, workload := range report.Observed.EligibleWorkloads {
		if workload.Namespace == referenceNamespace && workload.Container == referenceEligibleContainerName {
			return
		}
	}
	failf("missing eligible %q container workload: %#v", referenceEligibleContainerName, report.Observed.EligibleWorkloads)
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
