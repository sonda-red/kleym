package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRootHelpSucceeds(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected root help to succeed: %v", err)
	}
}

func TestRootVersionReportsLinkedVersion(t *testing.T) {
	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected --version to succeed: %v", err)
	}
	if got := stdout.String(); got != "dev\n" {
		t.Fatalf("expected linked version output %q, got %q", "dev\n", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr for --version, got:\n%s", stderr.String())
	}
}

func TestInspectBindingHelpIncludesMVPFlags(t *testing.T) {
	cmd := NewRootCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected inspect binding help to succeed: %v", err)
	}

	help := stdout.String() + "\n" + stderr.String()
	for _, want := range []string{"--namespace", "--output", "--strict", "--context", "--kubeconfig", "--timeout"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q\n%s", want, help)
		}
	}
	if !strings.Contains(help, `(default "default")`) {
		t.Fatalf("help output missing default namespace\n%s", help)
	}
}

func TestInspectBindingReturnsNotImplemented(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "binding", "my-binding", "-n", "default"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected inspect binding to return an error")
	}
	if !errors.Is(err, errInspectBindingNotImplemented) {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected command execution to leave error printing to main, got stderr:\n%s", stderr.String())
	}
}

func TestInvalidOutputRejected(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"inspect", "binding", "my-binding", "--output", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid output to fail")
	}
	if !strings.Contains(err.Error(), "invalid --output") {
		t.Fatalf("expected invalid --output error, got: %v", err)
	}
	if got := codeForError(err); got != exitUsage {
		t.Fatalf("expected invalid output to return exit code %d, got %d", exitUsage, got)
	}
}

func TestInvalidOutputNotRejectedForHelp(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--output", "yaml", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to bypass runnable output validation: %v", err)
	}
}

func TestInvalidTimeoutRejected(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"inspect", "binding", "my-binding", "--timeout", "0s"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid timeout to fail")
	}
	if !strings.Contains(err.Error(), "invalid --timeout") {
		t.Fatalf("expected invalid --timeout error, got: %v", err)
	}
	if got := codeForError(err); got != exitUsage {
		t.Fatalf("expected invalid timeout to return exit code %d, got %d", exitUsage, got)
	}
}

func TestUsageErrorsReturnUsageExitCode(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing binding name", args: []string{"inspect", "binding"}},
		{name: "invalid flag", args: []string{"inspect", "binding", "my-binding", "--not-a-flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected usage error")
			}
			if got := codeForError(err); got != exitUsage {
				t.Fatalf("expected usage error to return exit code %d, got %d", exitUsage, got)
			}
		})
	}
}

func TestExecuteMapsErrorsToExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{
			name: "success",
			args: []string{"--help"},
			want: exitOK,
		},
		{
			name: "version",
			args: []string{"--version"},
			want: exitOK,
		},
		{
			name:    "usage",
			args:    []string{"inspect", "binding", "my-binding", "--output", "yaml"},
			want:    exitUsage,
			wantErr: "invalid --output",
		},
		{
			name:    "unknown command",
			args:    []string{"nope"},
			want:    exitUsage,
			wantErr: "unknown command",
		},
		{
			name:    "internal",
			args:    []string{"inspect", "binding", "my-binding"},
			want:    exitInternal,
			wantErr: errInspectBindingNotImplemented.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			got := Execute(tt.args, stdout, stderr)
			if got != tt.want {
				t.Fatalf("expected exit code %d, got %d", tt.want, got)
			}
			if tt.wantErr == "" && stderr.Len() != 0 {
				t.Fatalf("expected no stderr, got:\n%s", stderr.String())
			}
			if tt.wantErr != "" && !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got:\n%s", tt.wantErr, stderr.String())
			}
		})
	}
}
