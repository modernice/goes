package cli

import (
	"github.com/modernice/goes/cli/internal/clifactory"
	"github.com/modernice/goes/cli/internal/cmd/rootcmd"
	"github.com/spf13/cobra"
)

// App is the CLI application.
type App struct {
	factory *clifactory.Factory
	root    *cobra.Command
}

// New returns the CLI App.
func New(opts ...clifactory.Option) *App {
	f := clifactory.New(opts...)
	return &App{
		factory: f,
		root:    rootcmd.New(f),
	}
}

// Factory returns the CLI Factory.
func (app *App) Factory() *clifactory.Factory {
	return app.factory
}

// Root returns the root command.
func (app *App) Root() *cobra.Command {
	return app.root
}

// Run runs the app.
func (app *App) Run() error {
	return app.root.Execute()
}
