package projectioncmd

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/logrusorgru/aurora"
	"github.com/modernice/goes/cli/internal/cliargs"
	"github.com/modernice/goes/cli/internal/clifactory"
	"github.com/modernice/goes/cli/internal/proto"
	"github.com/spf13/cobra"
)

var client proto.ProjectionServiceClient

// New returns the projection command.
func New(f *clifactory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projection",
		Short: "Manage projections",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			conn, err := f.Connect(f.Context)
			if err != nil {
				return err
			}
			client = proto.NewProjectionServiceClient(conn)
			return nil
		},
	}
	cmd.AddCommand(triggerCmd(f))

	return cmd
}

func triggerCmd(f *clifactory.Factory) *cobra.Command {
	var cfg struct {
		reset bool
	}

	cmd := &cobra.Command{
		Use:   "trigger <schedule> [<schedule> [<schedule>]]",
		Short: "Trigger projection schedules",
		Long: heredoc.Doc(`
			Trigger projection schedules through the projection service.

			Schedules that should be able to be triggered through the CLI must
			be registered with a name in a running projection.Service:

				var svc *projection.Service
				var schedule projection.Schedule
				svc.Register("example", schedule)

				errs, err := svc.Run(context.TODO())
		`),
		Example: heredoc.Doc(`
			Trigger "foo", "bar" & "baz" schedules:

			$ goes projection trigger foo bar baz

			Trigger "foo" schedule and reset projections before applying events:
			
			$ goes projection trigger foo --reset
		`),
		Args: cliargs.MinimumN(1, "Must provide at least one schedule name."),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, schedule := range args {
				if _, err := client.Trigger(f.Context, &proto.TriggerRequest{
					Schedule: schedule,
					Reset_:   cfg.reset,
				}); err != nil {
					return err
				}
			}

			cmd.Print(aurora.Green(heredoc.Docf(`
				Schedules triggered.

				Schedules: %v
				Reset:     %v
			`, args, cfg.reset)).String())

			return nil
		},
	}

	cmd.Flags().BoolVar(
		&cfg.reset, "reset", false,
		"Reset projections before applying events",
	)

	return cmd
}
