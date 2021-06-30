package rootcmd

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/modernice/goes/cli/internal/clifactory"
	"github.com/modernice/goes/cli/internal/cmd/projectioncmd"
	"github.com/spf13/cobra"
)

// New returns the root command.
func New(f *clifactory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "goes",
		Short:         "goes CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: heredoc.Doc(`
			$ goes projection trigger foo bar baz --reset
		`),
	}

	cmd.PersistentFlags().StringVar(
		&f.ConnectorAddress,
		"connect",
		f.ConnectorAddress,
		"CLI Connector address",
	)

	cmd.PersistentFlags().DurationVarP(
		&f.ConnectTimeout,
		"connect-timeout", "t",
		f.ConnectTimeout,
		"Timeout for connecting to CLI Connector",
	)

	cmd.AddCommand(projectioncmd.New(f))

	return cmd
}
