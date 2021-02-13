package cmdctx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/done"
	"github.com/modernice/goes/internal/xcommand/cmdctx"
)

type mockPayload struct{}

func TestWhenDone(t *testing.T) {
	cmd := command.New("foo", mockPayload{})

	ctx := cmdctx.New(cmd)

	if ctx.Command() != cmd {
		t.Errorf("ctx.Command should be %v; got %v", cmd, ctx.Command())
	}

	doneError := ctx.Done(context.Background())

	if doneError != nil {
		t.Errorf("ctx.Done() should not return an error; got %v", doneError)
	}
}

func TestWhenDone_withError(t *testing.T) {
	cmd := command.New("foo", mockPayload{})

	mockExecError := errors.New("mock exec error")
	mockDoneError := errors.New("mock done error")

	var cfg done.Config
	ctx := cmdctx.New(cmd, cmdctx.WhenDone(func(_ context.Context, opts ...done.Option) error {
		cfg = done.Configure(opts...)
		return mockDoneError
	}))

	if ctx.Command() != cmd {
		t.Errorf("ctx.Command should be %v; got %v", cmd, ctx.Command())
	}

	doneError := ctx.Done(context.Background(), done.WithError(mockExecError))

	if doneError != mockDoneError {
		t.Errorf("ctx.Done() should return %v; got %v", mockDoneError, doneError)
	}

	if cfg.Err != mockExecError {
		t.Errorf("cfg.Err should be %v; got %v", mockExecError, cfg.Err)
	}
}

func TestContext_Done_default(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	ctx := cmdctx.New(cmd)

	mockError := errors.New("mock error")
	err := ctx.Done(context.Background(), done.WithError(mockError))

	if err != nil {
		t.Errorf("ctx.Done() should return nil when WhenDone is not used")
	}
}
