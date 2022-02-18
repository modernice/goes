package cmdbus

import (
	"errors"
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/command"
)

// ExecutionError is the error returned by a Bus when doing a synchronous
// dispatch and the execution of the Command fails.
type ExecutionError[P any, ID goes.ID] struct {
	Cmd command.Of[P, ID]
	Err error
}

// ExecError unwraps err as an *ExecutionError.
func ExecError[P any, ID goes.ID](err error) (*ExecutionError[P, ID], bool) {
	var execError *ExecutionError[P, ID]
	if !errors.As(err, &execError) {
		return execError, false
	}
	return execError, true
}

func (err *ExecutionError[P, ID]) Error() string {
	return fmt.Sprintf("execute %q command: %v", err.Cmd.Name(), err.Err)
}

func (err *ExecutionError[P, ID]) Unwrap() error {
	return err.Err
}
