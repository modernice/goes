package action_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/saga/action"
)

func TestAction_Name(t *testing.T) {
	a := action.New("foo", func(action.Context[uuid.UUID]) error { return nil })
	if a.Name() != "foo" {
		t.Fatalf("Name() should return %q; got %q", "foo", a.Name())
	}
}

func TestAction_Run(t *testing.T) {
	var ctx action.Context[uuid.UUID]
	a := action.New("foo", func(action.Context[uuid.UUID]) error { return nil })
	if err := a.Run(ctx); err != nil {
		t.Errorf("Action should not fail; failed with %q", err)
	}
}

func TestAction_Run_error(t *testing.T) {
	var ctx action.Context[uuid.UUID]
	mockError := errors.New("mock error")
	a := action.New("foo", func(action.Context[uuid.UUID]) error { return mockError })

	err := a.Run(ctx)
	if err != mockError {
		t.Errorf("Action should fail with %q; got %q", mockError, err)
	}
}
