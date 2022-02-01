package cmdbus

import (
	"errors"
	"fmt"

	"github.com/modernice/goes/command"
)

// ExecutionError is the error returned by a Bus when doing a synchronous
// dispatch and the execution of the Command fails.
type ExecutionError[P any] struct {
	Cmd command.Of[P]
	Err error
}

// ExecError unwraps err as an *ExecutionError.
func ExecError[P any](err error) (*ExecutionError[P], bool) {
	var execError *ExecutionError[P]
	if !errors.As(err, &execError) {
		return execError, false
	}
	return execError, true
}

func (err *ExecutionError[P]) Error() string {
	return fmt.Sprintf("execute %q command: %v", err.Cmd.Name(), err.Err)
}

func (err *ExecutionError[P]) Unwrap() error {
	return err.Err
}
