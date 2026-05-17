package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

var errInspectBindingNotImplemented = errors.New("inspect binding is not implemented")

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
		RunE: func(_ *cobra.Command, _ []string) error {
			return withExitCode(exitInternal, errInspectBindingNotImplemented)
		},
	}

	inspectCmd.AddCommand(bindingCmd)
	return inspectCmd
}
