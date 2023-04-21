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

// Error returns a string representation of the error. The string includes the
// name of the command and the underlying error that caused the execution to fail.
func (err *ExecutionError[P]) Error() string {
	return fmt.Sprintf("execute %q command: %v", err.Cmd.Name(), err.Err)
}

// Unwrap returns the underlying error wrapped by *ExecutionError[P]. It
// implements the Unwrap method defined in the Go 1.13 error package
// [errors.Unwrap].
func (err *ExecutionError[P]) Unwrap() error {
	return err.Err
}
