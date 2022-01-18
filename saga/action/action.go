package action

// Action is a SAGA action.
type Action interface {
	// Name returns the name of the Action.
	Name() string

	// Run runs the Action.
	Run(Context) error
}

type action struct {
	name string
	run  func(Context) error
}

// New returns a new Action with the given name and runner.
func New(name string, run func(Context) error) Action {
	return &action{name, run}
}

func (a *action) Name() string {
	return a.name
}

func (a *action) Run(ctx Context) error {
	return a.run(ctx)
}
