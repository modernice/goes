package action

// Action is a SAGA action.
type Action[E, C any] interface {
	// Name returns the name of the Action.
	Name() string

	// Run runs the Action.
	Run(Context[E, C]) error
}

type action[E, C any] struct {
	name string
	run  func(Context[E, C]) error
}

// New returns a new Action with the given name and runner.
func New[E, C any](name string, run func(Context[E, C]) error) Action[E, C] {
	return &action[E, C]{name, run}
}

func (a *action[E, C]) Name() string {
	return a.name
}

func (a *action[E, C]) Run(ctx Context[E, C]) error {
	return a.run(ctx)
}
