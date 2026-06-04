package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sonda-red/kleym/internal/inspection"
	"github.com/sonda-red/kleym/internal/version"
	"github.com/spf13/cobra"
)

var errStatusHasErrorFindings = errors.New("status completed with error findings")

var newStatusRunner = func(opts *Options) (inspection.StatusInspector, error) {
	return inspection.NewKubernetesStatusInspector(inspection.Config{
		Context:    opts.Context,
		Kubeconfig: opts.Kubeconfig,
		Timeout:    opts.Timeout,
	})
}

// newStatusCommand creates the cluster overview command under the root CLI.
func newStatusCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Kleym cluster status",
		Args: func(cmd *cobra.Command, args []string) error {
			return withExitCode(exitUsage, cobra.NoArgs(cmd, args))
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateStatusOptions(opts)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := newStatusRunner(opts)
			if err != nil {
				return withExitCode(exitUsage, err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			report, statusErr := runner.Status(ctx)
			report.CLIVersion = version.Version
			code, err := statusExitCode(statusErr)
			report.ExitCode = &code
			if !shouldWriteStatusReport(statusErr) {
				if err != nil {
					return withExitCode(code, err)
				}
				return nil
			}
			if writeErr := inspection.WriteStatusReport(cmd.OutOrStdout(), opts.Output, report); writeErr != nil {
				return withExitCode(exitInternal, writeErr)
			}
			if err != nil {
				return withExitCode(code, err)
			}
			return nil
		},
	}
}

func validateStatusOptions(opts *Options) error {
	if !isValidOutputFormat(opts.Output) {
		return withExitCode(exitUsage, fmt.Errorf("invalid --output %q: must be one of %s", opts.Output, strings.Join(validOutputFormats, "|")))
	}
	if opts.Timeout <= 0 {
		return withExitCode(exitUsage, fmt.Errorf("invalid --timeout %s: must be greater than 0", opts.Timeout))
	}
	return nil
}

func shouldWriteStatusReport(statusErr error) bool {
	return statusErr == nil || errors.Is(statusErr, inspection.ErrStatusReportErrorFindings)
}

func statusExitCode(statusErr error) (int, error) {
	if statusErr == nil {
		return exitOK, nil
	}
	if errors.Is(statusErr, inspection.ErrStatusReportErrorFindings) {
		return exitInspectionIssue, errStatusHasErrorFindings
	}
	return exitUsage, statusErr
}
