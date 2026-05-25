package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sonda-red/kleym/internal/inspection"
	"sigs.k8s.io/yaml"
)

func TestInspectBindingJSONUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := inspection.NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = inspection.BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a"}
	newBindingInspectionRunner = func(_ *Options) (inspection.BindingInspector, error) {
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

	var got inspection.BindingInspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal inspect output: %v\n%s", err, stdout.String())
	}
	if got.BindingRef.Namespace != "tenant-a" || got.BindingRef.Name != "binding-a" {
		t.Fatalf("bindingRef = %#v, want tenant-a/binding-a", got.BindingRef)
	}
}

func TestInspectBindingYAMLUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := inspection.NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = inspection.BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a"}
	newBindingInspectionRunner = func(_ *Options) (inspection.BindingInspector, error) {
		return fixedInspectionRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "binding-a", "-n", "tenant-a", "-o", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect binding returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}

	yamlAsJSON, err := yaml.YAMLToJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("convert inspect YAML output: %v\n%s", err, stdout.String())
	}
	var got inspection.BindingInspectionReport
	if err := json.Unmarshal(yamlAsJSON, &got); err != nil {
		t.Fatalf("unmarshal inspect YAML output: %v\n%s", err, stdout.String())
	}
	if got.BindingRef.Namespace != "tenant-a" || got.BindingRef.Name != "binding-a" {
		t.Fatalf("bindingRef = %#v, want tenant-a/binding-a", got.BindingRef)
	}
}

func TestInspectBindingMarkdownUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := inspection.NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = inspection.BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a", Mode: "PerObjective"}
	fakeReport.Desired = inspection.BindingInspectionDesiredState{
		ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
		SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
	}
	newBindingInspectionRunner = func(_ *Options) (inspection.BindingInspector, error) {
		return fixedInspectionRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "binding-a", "-n", "tenant-a", "-o", "markdown"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect binding returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"# BindingInspectionReport",
		"| Name | tenant-a/binding-a |",
		"| Mode | PerObjective |",
		"| ClusterSPIFFEID | tenant-a-binding-a-1234abcd |",
		"| SPIFFE ID | spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a |",
		"<summary>Canonical JSON report</summary>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown output missing %q\n%s", want, out)
		}
	}
}

func TestInspectBindingDefaultTextUsesRunner(t *testing.T) {
	originalFactory := newBindingInspectionRunner
	t.Cleanup(func() { newBindingInspectionRunner = originalFactory })

	fakeReport := inspection.NewBindingInspectionReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.BindingRef = inspection.BindingInspectionBindingRef{Namespace: "tenant-a", Name: "binding-a", Mode: "PerObjective"}
	fakeReport.Desired = inspection.BindingInspectionDesiredState{
		ClusterSPIFFEIDName: "tenant-a-binding-a-1234abcd",
		SPIFFEID:            "spiffe://kleym.sonda.red/ns/tenant-a/objective/objective-a",
	}
	newBindingInspectionRunner = func(_ *Options) (inspection.BindingInspector, error) {
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
		report     inspection.BindingInspectionReport
		inspectErr error
		strict     bool
		wantCode   int
		wantErr    error
	}{
		{
			name:     "success",
			report:   inspection.NewBindingInspectionReport(),
			wantCode: exitOK,
		},
		{
			name: "warning is non-fatal without strict",
			report: inspection.BindingInspectionReport{Findings: []inspection.BindingInspectionFinding{{
				ID:       "observed-drift",
				Severity: inspection.BindingInspectionFindingSeverityWarning,
				Reason:   "ObservedDrift",
				Message:  "drift",
			}}},
			wantCode: exitOK,
		},
		{
			name: "warning is inspection issue with strict",
			report: inspection.BindingInspectionReport{Findings: []inspection.BindingInspectionFinding{{
				ID:       "observed-drift",
				Severity: inspection.BindingInspectionFindingSeverityWarning,
				Reason:   "ObservedDrift",
				Message:  "drift",
			}}},
			strict:   true,
			wantCode: exitInspectionIssue,
			wantErr:  errInspectBindingHasWarningFindings,
		},
		{
			name: "error finding",
			report: inspection.BindingInspectionReport{Findings: []inspection.BindingInspectionFinding{{
				ID:       "dependency-missing",
				Severity: inspection.BindingInspectionFindingSeverityError,
				Reason:   "Missing",
				Message:  "missing",
			}}},
			inspectErr: inspection.ErrBindingInspectionErrorFindings,
			wantCode:   exitInspectionIssue,
			wantErr:    errInspectBindingHasErrorFindings,
		},
		{
			name:       "binding not found",
			report:     inspection.NewBindingInspectionReport(),
			inspectErr: inspection.ErrBindingInspectionNotFound,
			wantCode:   exitBindingNotFound,
			wantErr:    inspection.ErrBindingInspectionNotFound,
		},
		{
			name:       "inspection setup failure",
			report:     inspection.NewBindingInspectionReport(),
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

type fixedInspectionRunner struct {
	report inspection.BindingInspectionReport
	err    error
}

func (r fixedInspectionRunner) InspectBinding(_ context.Context, _ string, _ string) (inspection.BindingInspectionReport, error) {
	return r.report, r.err
}

var _ inspection.BindingInspector = fixedInspectionRunner{}
