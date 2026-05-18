package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	exitOK              = 0
	exitGeneric         = 1
	exitInspectionIssue = 2
	exitBindingNotFound = 3
	exitUsage           = 4
	exitInternal        = 5
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

// Execute runs the CLI and returns the process exit code defined by docs/spec/cli.md.
func Execute(args []string, stdout io.Writer, stderr io.Writer) int {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return codeForError(err)
	}
	return exitOK
}

func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

func codeForError(err error) int {
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	if strings.HasPrefix(err.Error(), "unknown command ") {
		return exitUsage
	}
	return exitGeneric
}
