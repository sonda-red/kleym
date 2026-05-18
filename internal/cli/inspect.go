package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

var (
	errInspectBindingTextOutputUnsupported = errors.New("binding inspection text output is not implemented")
	errInspectBindingHasErrorFindings      = errors.New("binding inspection completed with error findings")
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
			if opts.Output != outputJSON {
				return withExitCode(exitInternal, errInspectBindingTextOutputUnsupported)
			}

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
			if inspectErr != nil {
				if errors.Is(inspectErr, errBindingInspectionNotFound) {
					return withExitCode(exitBindingNotFound, inspectErr)
				}
				if errors.Is(inspectErr, errBindingInspectionErrorFindings) {
					return withExitCode(exitInspectionIssue, errInspectBindingHasErrorFindings)
				}
				return withExitCode(exitUsage, inspectErr)
			}
			return nil
		},
	}

	inspectCmd.AddCommand(bindingCmd)
	return inspectCmd
}
