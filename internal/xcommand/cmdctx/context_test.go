package cmdctx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/internal/xcommand/cmdctx"
)

type mockPayload struct{}

func TestWhenDone(t *testing.T) {
	cmd := command.New("foo", mockPayload{})

	mockExecError := errors.New("mock exec error")
	mockDoneError := errors.New("mock done error")

	var execError error
	ctx := cmdctx.New(cmd, cmdctx.WhenDone(func(_ context.Context, err error) error {
		execError = err
		return mockDoneError
	}))

	if ctx.Command() != cmd {
		t.Errorf("ctx.Command should be %v; got %v", cmd, ctx.Command())
	}

	doneError := ctx.Done(context.Background(), mockExecError)
	if doneError != mockDoneError {
		t.Errorf("ctx.Done() should return %v; got %v", mockDoneError, doneError)
	}

	if execError != mockExecError {
		t.Errorf("Context received wrong execution error. want=%v got=%v", mockExecError, execError)
	}
}

func TestContext_Done_default(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	ctx := cmdctx.New(cmd)

	mockError := errors.New("mock error")
	err := ctx.Done(context.Background(), mockError)

	if err != nil {
		t.Errorf("ctx.Done() should return nil when WhenDone is not used")
	}
}
