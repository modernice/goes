package cmdbus

import (
	"errors"
	"fmt"

	"github.com/modernice/goes/command"
)

// ExecutionError is the error returned by a Bus when doing a synchronous
// dispatch and the execution of the Command fails.
type ExecutionError struct {
	Cmd command.Command
	Err error
}

// ExecError unwraps err as an *ExecutionError.
func ExecError(err error) (*ExecutionError, bool) {
	var execError *ExecutionError
	if !errors.As(err, &execError) {
		return execError, false
	}
	return execError, true
}

func (err *ExecutionError) Error() string {
	return fmt.Sprintf("execute %q command: %v", err.Cmd.Name(), err.Err)
}

func (err *ExecutionError) Unwrap() error {
	return err.Err
}
