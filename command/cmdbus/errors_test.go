package cmdbus_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
)

func TestExecError(t *testing.T) {
	execError := &cmdbus.ExecutionError[any]{}

	err, ok := cmdbus.ExecError[any](execError)
	if !ok {
		t.Fatalf("ExecError() should return true for %v; got %t", execError, ok)
	}

	if err != execError {
		t.Fatalf("ExecError() should return %v; got %v", execError, err)
	}
}

func TestExecutionError_Error(t *testing.T) {
	err := &cmdbus.ExecutionError[any]{
		Cmd: command.New("foo", mockPayload{}).Any(),
		Err: errors.New("mock error"),
	}

	want := fmt.Sprintf("execute %q command: %v", err.Cmd.Name(), err.Err)
	if err.Error() != want {
		t.Errorf("err.Error() should return %q; got %q", want, err.Error())
	}
}
