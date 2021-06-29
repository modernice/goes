package cliargs

import (
	"errors"

	"github.com/spf13/cobra"
)

// MinimumN returns a cobra.PositionalArgs that requires at least n arguments
// to be passed.
func MinimumN(n int, errmsg string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < n {
			return errors.New(errmsg)
		}
		return nil
	}
}
