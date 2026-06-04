package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/sonda-red/kleym/internal/identity"
	"github.com/sonda-red/kleym/internal/inspection"
	"github.com/sonda-red/kleym/internal/version"
	"github.com/spf13/cobra"
)

const (
	defaultNamespace = "default"
	outputText       = "text"
	outputJSON       = "json"
)

var validOutputFormats = []string{outputText, outputJSON}

// Options holds top-level CLI flags for the kleym command tree.
type Options struct {
	Namespace                        string
	Output                           string
	Strict                           bool
	Context                          string
	Kubeconfig                       string
	Timeout                          time.Duration
	TrustDomain                      string
	TrustDomainOverride              bool
	ClusterSPIFFEIDClassName         string
	ClusterSPIFFEIDClassNameOverride bool
}

// NewRootCommand builds the kleym root command defined by the CLI spec.
func NewRootCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:           "kleym",
		Short:         "Read-only inspection CLI for Kleym bindings",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.Version,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return withExitCode(exitUsage, err)
	})
	cmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")

	cmd.PersistentFlags().StringVarP(&opts.Namespace, "namespace", "n", defaultNamespace, "Namespace for binding lookup")
	cmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", outputText, "Output format: "+strings.Join(validOutputFormats, "|"))
	cmd.PersistentFlags().BoolVar(&opts.Strict, "strict", false, "Treat warning findings as errors")
	cmd.PersistentFlags().StringVar(&opts.Context, "context", "", "Name of the kubeconfig context to use")
	cmd.PersistentFlags().StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "Inspection timeout")
	cmd.PersistentFlags().StringVar(&opts.TrustDomain, "trust-domain", identity.DefaultTrustDomain, "SPIRE Server trust domain used when recomputing desired SPIFFE IDs")
	cmd.PersistentFlags().StringVar(&opts.ClusterSPIFFEIDClassName, "clusterspiffeid-class-name", "", "Optional SPIRE Controller Manager ClusterSPIFFEID className expected on managed resources")

	cmd.AddCommand(newInspectCommand(opts))
	return cmd
}

// captureIdentityConfigFlagSources records whether identity config flags were
// explicitly set so inspection can prefer binding status by default.
func captureIdentityConfigFlagSources(cmd *cobra.Command, opts *Options) {
	if flag := cmd.Flag("trust-domain"); flag != nil {
		opts.TrustDomainOverride = flag.Changed
	}
	if flag := cmd.Flag("clusterspiffeid-class-name"); flag != nil {
		opts.ClusterSPIFFEIDClassNameOverride = flag.Changed
	}
}

func validateRunnableOptions(opts *Options) error {
	if !isValidOutputFormat(opts.Output) {
		return withExitCode(exitUsage, fmt.Errorf("invalid --output %q: must be one of %s", opts.Output, strings.Join(validOutputFormats, "|")))
	}
	if opts.Timeout <= 0 {
		return withExitCode(exitUsage, fmt.Errorf("invalid --timeout %s: must be greater than 0", opts.Timeout))
	}
	if err := inspection.ValidateOperatorIdentityConfig(opts.TrustDomain, opts.ClusterSPIFFEIDClassName); err != nil {
		return withExitCode(exitUsage, err)
	}
	return nil
}

func isValidOutputFormat(output string) bool {
	for _, valid := range validOutputFormats {
		if output == valid {
			return true
		}
	}
	return false
}
