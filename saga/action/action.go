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

// Name returns the name of a SAGA action. It is a method of the Action
// interface, which is implemented by the action struct. To create a new Action,
// use the New function which takes a name and a runner function as arguments.
func (a *action) Name() string {
	return a.name
}

// Run executes the Action and returns an error if any. It takes a Context
// parameter [Context] and returns an error.
func (a *action) Run(ctx Context) error {
	return a.run(ctx)
}
