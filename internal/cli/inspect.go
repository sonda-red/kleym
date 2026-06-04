package cli

import (
	"context"
	"errors"

	"github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/inspection"
	"github.com/spf13/cobra"
)

var (
	errInspectBindingHasErrorFindings   = errors.New("binding inspection completed with error findings")
	errInspectBindingHasWarningFindings = errors.New("binding inspection completed with warning findings in strict mode")
)

var newBindingInspectionRunner = func(opts *Options) (inspection.BindingInspector, error) {
	return inspection.NewKubernetesBindingInspector(inspection.Config{
		Context:                          opts.Context,
		Kubeconfig:                       opts.Kubeconfig,
		Timeout:                          opts.Timeout,
		TrustDomain:                      opts.TrustDomain,
		TrustDomainOverride:              opts.TrustDomainOverride,
		ClusterSPIFFEIDClassName:         opts.ClusterSPIFFEIDClassName,
		ClusterSPIFFEIDClassNameOverride: opts.ClusterSPIFFEIDClassNameOverride,
	})
}

// newInspectCommand creates the inspect command group under the root CLI.
func newInspectCommand(opts *Options) *cobra.Command {
	inspectCmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect Kleym resources",
	}
	inspectCmd.PersistentFlags().StringVarP(&opts.Namespace, "namespace", "n", defaultNamespace, "Namespace for binding lookup")
	inspectCmd.PersistentFlags().BoolVar(&opts.Strict, "strict", false, "Treat warning findings as errors")
	inspectCmd.PersistentFlags().StringVar(&opts.TrustDomain, "trust-domain", identity.DefaultTrustDomain, "SPIRE Server trust domain used when rendering SPIFFE IDs")
	inspectCmd.PersistentFlags().StringVar(&opts.ClusterSPIFFEIDClassName, "clusterspiffeid-class-name", "", "Optional SPIRE Controller Manager ClusterSPIFFEID className expected on managed resources")

	bindingCmd := &cobra.Command{
		Use:   "binding <name>",
		Short: "Inspect one InferenceIdentityBinding",
		Args: func(cmd *cobra.Command, args []string) error {
			return withExitCode(exitUsage, cobra.ExactArgs(1)(cmd, args))
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			captureIdentityConfigFlagSources(cmd, opts)
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
			code, err := inspectionExitCode(report, inspectErr, opts.Strict)
			report.ExitCode = &code
			if !shouldWriteBindingInspectionReport(inspectErr) {
				if err != nil {
					return withExitCode(code, err)
				}
				return nil
			}
			if writeErr := inspection.WriteBindingInspectionReport(cmd.OutOrStdout(), opts.Output, report); writeErr != nil {
				return withExitCode(exitInternal, writeErr)
			}
			if err != nil {
				return withExitCode(code, err)
			}
			return nil
		},
	}

	inspectCmd.AddCommand(bindingCmd)
	return inspectCmd
}

func shouldWriteBindingInspectionReport(inspectErr error) bool {
	if inspectErr == nil {
		return true
	}
	return errors.Is(inspectErr, inspection.ErrBindingInspectionNotFound) ||
		errors.Is(inspectErr, inspection.ErrBindingInspectionErrorFindings)
}

func inspectionExitCode(report inspection.BindingInspectionReport, inspectErr error, strict bool) (int, error) {
	if inspectErr != nil {
		if errors.Is(inspectErr, inspection.ErrBindingInspectionNotFound) {
			return exitBindingNotFound, inspectErr
		}
		if errors.Is(inspectErr, inspection.ErrBindingInspectionErrorFindings) {
			return exitInspectionIssue, errInspectBindingHasErrorFindings
		}
		return exitUsage, inspectErr
	}
	if inspection.HasErrorSeverityFinding(report.Findings) {
		return exitInspectionIssue, errInspectBindingHasErrorFindings
	}
	if strict && inspection.HasWarningSeverityFinding(report.Findings) {
		return exitInspectionIssue, errInspectBindingHasWarningFindings
	}
	return exitOK, nil
}
