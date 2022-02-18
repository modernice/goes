package action

import "github.com/modernice/goes"

// Action is a SAGA action.
type Action[ID goes.ID] interface {
	// Name returns the name of the Action.
	Name() string

	// Run runs the Action.
	Run(Context[ID]) error
}

type action[ID goes.ID] struct {
	name string
	run  func(Context[ID]) error
}

// New returns a new Action with the given name and runner.
func New[ID goes.ID](name string, run func(Context[ID]) error) Action[ID] {
	return &action[ID]{name, run}
}

func (a *action[ID]) Name() string {
	return a.name
}

func (a *action[ID]) Run(ctx Context[ID]) error {
	return a.run(ctx)
}
