package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/finish"
)

// type mockPayload struct{}

func TestWhenDone(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	ctx := command.NewContext[mockPayload](context.Background(), cmd)

	doneError := ctx.Finish(context.Background())

	if doneError != nil {
		t.Errorf("ctx.Done() should not return an error; got %v", doneError)
	}
}

func TestWhenDone_withError(t *testing.T) {
	cmd := command.New("foo", mockPayload{})

	mockExecError := errors.New("mock exec error")
	mockDoneError := errors.New("mock done error")

	var cfg finish.Config
	ctx := command.NewContext[mockPayload](context.Background(), cmd, command.WhenDone(func(_ context.Context, cfg2 finish.Config) error {
		cfg = cfg2
		return mockDoneError
	}))

	doneError := ctx.Finish(context.Background(), finish.WithError(mockExecError))

	if doneError != mockDoneError {
		t.Errorf("ctx.Done() should return %v; got %v", mockDoneError, doneError)
	}

	if cfg.Err != mockExecError {
		t.Errorf("cfg.Err should be %v; got %v", mockExecError, cfg.Err)
	}
}

func TestContext_Finish_default(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	ctx := command.NewContext[mockPayload](context.Background(), cmd)

	mockError := errors.New("mock error")
	err := ctx.Finish(context.Background(), finish.WithError(mockError))

	if err != nil {
		t.Errorf("ctx.Done() should return nil when WhenDone is not used")
	}
}

func TestContext_Finish_multipleTimes(t *testing.T) {
	cmd := command.New("foo", mockPayload{})
	var doneCount int
	ctx := command.NewContext[mockPayload](
		context.Background(),
		cmd,
		command.WhenDone(func(_ context.Context, _ finish.Config) error {
			doneCount++
			return nil
		}),
	)

	err := ctx.Finish(context.Background())

	if err != nil {
		t.Fatalf("Finish() shouldn't fail; failed with %q", err)
	}

	if err = ctx.Finish(context.Background()); err != nil {
		t.Fatalf("Finish() shouldn't fail if called multiple times")
	}

	if doneCount != 1 {
		t.Fatalf("WhenDone() callback should have been called once; was called %d times", doneCount)
	}
}
