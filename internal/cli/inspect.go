package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

var (
	errInspectBindingHasErrorFindings   = errors.New("binding inspection completed with error findings")
	errInspectBindingHasWarningFindings = errors.New("binding inspection completed with warning findings in strict mode")
)

// newInspectCommand creates the inspect command group under the root CLI.
func newInspectCommand(opts *Options) *cobra.Command {
	inspectCmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect Kleym resources",
	}

	bindingCmd := &cobra.Command{
		Use:   "binding <name>",
		Short: "Inspect one InferenceIdentityBinding",
		Args: func(cmd *cobra.Command, args []string) error {
			return withExitCode(exitUsage, cobra.ExactArgs(1)(cmd, args))
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateRunnableOptions(opts)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector, err := newBindingInspectionRunner(opts)
			if err != nil {
				return withExitCode(exitUsage, err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			report, inspectErr := inspector.InspectBinding(ctx, opts.Namespace, args[0])
			if err := WriteBindingInspectionReport(cmd.OutOrStdout(), opts.Output, report); err != nil {
				return withExitCode(exitInternal, err)
			}
			code, err := inspectionExitCode(report, inspectErr, opts.Strict)
			if err != nil {
				return withExitCode(code, err)
			}
			return nil
		},
	}

	inspectCmd.AddCommand(bindingCmd)
	return inspectCmd
}

func inspectionExitCode(report BindingInspectionReport, inspectErr error, strict bool) (int, error) {
	if inspectErr != nil {
		if errors.Is(inspectErr, errBindingInspectionNotFound) {
			return exitBindingNotFound, inspectErr
		}
		if errors.Is(inspectErr, errBindingInspectionErrorFindings) {
			return exitInspectionIssue, errInspectBindingHasErrorFindings
		}
		return exitUsage, inspectErr
	}
	if hasErrorSeverityFinding(report.Findings) {
		return exitInspectionIssue, errInspectBindingHasErrorFindings
	}
	if strict && hasWarningSeverityFinding(report.Findings) {
		return exitInspectionIssue, errInspectBindingHasWarningFindings
	}
	return exitOK, nil
}
