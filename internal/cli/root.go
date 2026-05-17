package cli

import (
	"fmt"
	"time"

	"github.com/sonda-red/kleym/internal/version"
	"github.com/spf13/cobra"
)

const (
	defaultNamespace = "default"
	outputText       = "text"
	outputJSON       = "json"
)

// Options holds top-level CLI flags for the kleym command tree.
type Options struct {
	Namespace  string
	Output     string
	Strict     bool
	Context    string
	Kubeconfig string
	Timeout    time.Duration
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
	cmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", outputText, "Output format: text|json")
	cmd.PersistentFlags().BoolVar(&opts.Strict, "strict", false, "Treat warning findings as errors")
	cmd.PersistentFlags().StringVar(&opts.Context, "context", "", "Name of the kubeconfig context to use")
	cmd.PersistentFlags().StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "Inspection timeout")

	cmd.AddCommand(newInspectCommand(opts))
	return cmd
}

func validateRunnableOptions(opts *Options) error {
	if opts.Output != outputText && opts.Output != outputJSON {
		return withExitCode(exitUsage, fmt.Errorf("invalid --output %q: must be one of %s|%s", opts.Output, outputText, outputJSON))
	}
	if opts.Timeout <= 0 {
		return withExitCode(exitUsage, fmt.Errorf("invalid --timeout %s: must be greater than 0", opts.Timeout))
	}
	return nil
}
