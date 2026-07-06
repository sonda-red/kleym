package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sonda-red/kleym/internal/inspection"
)

func TestStatusJSONUsesRunner(t *testing.T) {
	originalFactory := newStatusRunner
	t.Cleanup(func() { newStatusRunner = originalFactory })

	fakeReport := inspection.NewStatusReport()
	fakeReport.GeneratedAt = "2026-05-18T10:11:12Z"
	fakeReport.Status = inspection.StatusResultOK
	fakeReport.Components.Kleym.Status = inspection.StatusResultOK
	newStatusRunner = func(_ *Options) (inspection.StatusInspector, error) {
		return fixedStatusRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"status", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}

	var got inspection.StatusReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal status output: %v\n%s", err, stdout.String())
	}
	if got.Kind != inspection.StatusReportKind || got.Status != inspection.StatusResultOK {
		t.Fatalf("status report = %#v, want OK KleymStatusReport", got)
	}
}

func TestStatusDefaultTextUsesRunner(t *testing.T) {
	originalFactory := newStatusRunner
	t.Cleanup(func() { newStatusRunner = originalFactory })

	fakeReport := inspection.NewStatusReport()
	fakeReport.Status = inspection.StatusResultOK
	fakeReport.Components.Kleym.Status = inspection.StatusResultOK
	fakeReport.Components.Operator = inspection.OperatorStatus{
		Status:        inspection.StatusResultOK,
		Deployment:    "kleym-system/kleym-operator",
		ReadyReplicas: 1,
		Replicas:      1,
		Version:       "v0.3.0",
	}
	fakeReport.Components.KleymCRDs = inspection.KleymAPIStatus{
		Status:                   inspection.StatusResultOK,
		InferenceIdentityBinding: "v1alpha1",
	}
	fakeReport.Components.SPIRECRDs = inspection.SPIRECRDStatus{
		Status:          inspection.StatusResultOK,
		ClusterSPIFFEID: "v1alpha1",
	}
	fakeReport.Components.GAIECRDs = inspection.GAIEStatus{
		Status:        inspection.StatusResultOK,
		InferencePool: "v1",
	}
	fakeReport.Config.TrustDomain = "example.org"
	fakeReport.Config.ClusterSPIFFEIDClassName = "kleym"
	fakeReport.Config.ClusterSPIFFEIDClassNameKnown = true
	fakeReport.Summary.Bindings.Total = 1
	fakeReport.Summary.Bindings.Conditions.Ready = 1
	newStatusRunner = func(_ *Options) (inspection.StatusInspector, error) {
		return fixedStatusRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"Kleym",
		"  CLI: dev",
		"  Operator: Available",
		"    Deployment: kleym-system/kleym-operator",
		"    Ready: 1/1",
		"    Version: v0.3.0",
		"    trustDomain: example.org",
		"    clusterSPIFFEIDClass: kleym",
		"    InferenceIdentityBinding: v1alpha1",
		"Bindings",
		"  Total: 1",
		"    Ready: 1",
		"Dependencies",
		"  GAIE: Available",
		"    InferencePool: v1",
		"  SPIRE: Available",
		"    ClusterSPIFFEID: v1alpha1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}
}

func TestStatusTextShowsUnknownClassWhenConfigUnavailable(t *testing.T) {
	originalFactory := newStatusRunner
	t.Cleanup(func() { newStatusRunner = originalFactory })

	fakeReport := inspection.NewStatusReport()
	fakeReport.Status = inspection.StatusResultError
	fakeReport.Components.Operator.Status = inspection.StatusResultError
	newStatusRunner = func(_ *Options) (inspection.StatusInspector, error) {
		return fixedStatusRunner{report: fakeReport}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "    trustDomain: unknown") ||
		!strings.Contains(out, "    clusterSPIFFEIDClass: unknown") {
		t.Fatalf("text output did not show unknown config\n%s", out)
	}
}

func TestStatusErrorFindingsWriteReport(t *testing.T) {
	originalFactory := newStatusRunner
	t.Cleanup(func() { newStatusRunner = originalFactory })

	fakeReport := inspection.NewStatusReport()
	fakeReport.Status = inspection.StatusResultError
	fakeReport.Findings = []inspection.BindingInspectionFinding{{
		ID:       "crd-missing",
		Severity: inspection.BindingInspectionFindingSeverityError,
		Reason:   "ClusterSPIFFEIDCRDMissing",
		Message:  "ClusterSPIFFEID CRD is not installed",
	}}
	newStatusRunner = func(_ *Options) (inspection.StatusInspector, error) {
		return fixedStatusRunner{report: fakeReport, err: inspection.ErrStatusReportErrorFindings}, nil
	}

	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"status", "-o", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected status to return an error")
	}
	if !errors.Is(err, errStatusHasErrorFindings) {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := codeForError(err); got != exitInspectionIssue {
		t.Fatalf("exit code = %d, want %d", got, exitInspectionIssue)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected status report on stdout")
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got:\n%s", stderr.String())
	}
}

func TestStatusRejectsInspectOnlyFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "namespace",
			args:    []string{"status", "-n", "tenant-a"},
			wantErr: "unknown shorthand flag: 'n'",
		},
		{
			name:    "strict",
			args:    []string{"status", "--strict"},
			wantErr: "unknown flag: --strict",
		},
		{
			name:    "trust domain",
			args:    []string{"status", "--trust-domain", "example.org"},
			wantErr: "unknown flag: --trust-domain",
		},
		{
			name:    "clusterspiffeid class",
			args:    []string{"status", "--clusterspiffeid-class-name", "kleym"},
			wantErr: "unknown flag: --clusterspiffeid-class-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected status to reject inspect-only flag")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if got := codeForError(err); got != exitUsage {
				t.Fatalf("exit code = %d, want %d", got, exitUsage)
			}
		})
	}
}

func TestStatusExitCodeMapping(t *testing.T) {
	setupErr := errors.New("load Kubernetes config")
	tests := []struct {
		name      string
		statusErr error
		wantCode  int
		wantErr   error
	}{
		{name: "success", wantCode: exitOK},
		{
			name:      "error findings",
			statusErr: inspection.ErrStatusReportErrorFindings,
			wantCode:  exitInspectionIssue,
			wantErr:   errStatusHasErrorFindings,
		},
		{
			name:      "setup failure",
			statusErr: setupErr,
			wantCode:  exitUsage,
			wantErr:   setupErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotErr := statusExitCode(tt.statusErr)
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

type fixedStatusRunner struct {
	report inspection.StatusReport
	err    error
}

func (r fixedStatusRunner) Status(_ context.Context) (inspection.StatusReport, error) {
	return r.report, r.err
}

var _ inspection.StatusInspector = fixedStatusRunner{}
