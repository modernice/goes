package saga_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/modernice/goes/saga"
	"github.com/modernice/goes/saga/action"
	"github.com/modernice/goes/saga/report"
)

type recorder struct {
	count map[string]int
	chain []string
}

func TestExecute_implicitStartAction(t *testing.T) {
	var called bool
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			called = true
			return nil
		}),
		saga.Action[any, any]("bar", func(action.Context[any, any]) error {
			return nil
		}),
	)

	if err := saga.Execute(context.Background(), s); err != nil {
		t.Errorf("SAGA shouldn't fail; failed with %q", err)
	}

	if !called {
		t.Errorf("first configured Action should have been called implicitly!")
	}
}

func TestExecute_explicitStartAction(t *testing.T) {
	var called bool
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			return nil
		}),
		saga.Action[any, any]("bar", func(action.Context[any, any]) error {
			called = true
			return nil
		}),
		saga.StartWith[any, any]("bar"),
	)

	if err := saga.Execute(context.Background(), s); err != nil {
		t.Errorf("SAGA shouldn't fail; failed with %q", err)
	}

	if !called {
		t.Errorf("configured starting Action should have been called!")
	}
}

func TestExecute_actionError(t *testing.T) {
	mockError := errors.New("mock error")
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			return mockError
		}),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, mockError) {
		t.Errorf("SAGA should fail with %q; got %q", mockError, err)
	}
}

func TestExecute_compensate(t *testing.T) {
	mockError := errors.New("mock error")
	var compensated bool
	s := saga.New(
		saga.Action[any, any]("foo", func(c action.Context[any, any]) error {
			return nil
		}),
		saga.Action[any, any]("bar", func(action.Context[any, any]) error {
			return mockError
		}),
		saga.Action[any, any]("comp-foo", func(action.Context[any, any]) error {
			compensated = true
			return nil
		}),
		saga.Sequence[any, any]("foo", "bar"),
		saga.Compensate[any, any]("foo", "comp-foo"),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, mockError) {
		t.Fatalf("SAGA should fail with %q; got %q", mockError, err)
	}

	if !compensated {
		t.Errorf("compensating Action not called!")
	}
}

func TestExecute_compensateError(t *testing.T) {
	mockError := errors.New("mock error")
	mockCompError := errors.New("mock comp error")
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			return nil
		}),
		saga.Action[any, any]("bar", func(action.Context[any, any]) error {
			return mockError
		}),
		saga.Action[any, any]("baz", func(action.Context[any, any]) error {
			return mockCompError
		}),
		saga.Sequence[any, any]("foo", "bar"),
		saga.Compensate[any, any]("foo", "baz"),
	)

	err := saga.Execute(context.Background(), s)
	if !errors.Is(err, mockCompError) {
		t.Errorf("SAGA should fail with %q; got %q", mockCompError, err)
	}

	var compError *saga.CompensateErr
	if !errors.As(err, &compError) {
		t.Fatalf("SAGA should fail with a %T error; got %T", compError, err)
	}

	compError, ok := saga.CompensateError(err)
	if !ok {
		t.Fatalf("SAGA should fail with a %T error; got %T", compError, err)
	}

	if compError.ActionError != mockError {
		t.Fatalf("CompensateError.ActionError should be %q; got %q", mockError, compError.ActionError)
	}
}

func TestExecute_compensateChain(t *testing.T) {
	mockError := errors.New("mock error")

	rec := newRecorder()
	s := saga.New(
		rec.newAction("foo", func(c action.Context[any, any]) error {
			c.Run(c, "bar")
			return mockError
		}),
		rec.newAction("bar", func(c action.Context[any, any]) error {
			return c.Run(c, "baz")
		}),
		rec.newAction("baz", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-foo", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-bar", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-baz", func(c action.Context[any, any]) error {
			return nil
		}),
		saga.Compensate[any, any]("foo", "comp-foo"),
		saga.Compensate[any, any]("bar", "comp-bar"),
		saga.Compensate[any, any]("baz", "comp-baz"),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, mockError) {
		t.Errorf("SAGA should fail with %q; got %q", mockError, err)
	}

	want := []string{
		"baz", "bar", "foo",
		"comp-bar", "comp-baz",
	}
	if !reflect.DeepEqual(rec.chain, want) {
		t.Errorf("Actions should be called in order. want=%s got=%s", want, rec.chain)
	}
}

func TestExecute_compensateChainError(t *testing.T) {
	mockError := errors.New("mock error")
	mockCompError := errors.New("mock comp error")
	rec := newRecorder()
	s := saga.New(
		rec.newAction("foo", func(c action.Context[any, any]) error {
			c.Run(c, "bar")
			return mockError
		}),
		rec.newAction("bar", func(c action.Context[any, any]) error {
			return c.Run(c, "baz")
		}),
		rec.newAction("baz", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-foo", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-bar", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-baz", func(c action.Context[any, any]) error {
			return mockCompError
		}),
		saga.Compensate[any, any]("foo", "comp-foo"),
		saga.Compensate[any, any]("bar", "comp-bar"),
		saga.Compensate[any, any]("baz", "comp-baz"),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, mockCompError) {
		t.Errorf("SAGA should fail with %q; got %q", mockCompError, err)
	}

	want := []string{
		"baz", "bar", "foo",
		"comp-bar", "comp-baz",
	}
	if !reflect.DeepEqual(rec.chain, want) {
		t.Errorf("Actions should be called in order. want=%s got=%s", want, rec.chain)
	}
}

func TestExecute_compensateChainErrorMiddle(t *testing.T) {
	mockError := errors.New("mock error")
	mockCompError := errors.New("mock comp error")
	rec := newRecorder()
	s := saga.New(
		rec.newAction("foo", func(c action.Context[any, any]) error {
			c.Run(c, "bar")
			return mockError
		}),
		rec.newAction("bar", func(c action.Context[any, any]) error {
			return c.Run(c, "baz")
		}),
		rec.newAction("baz", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-foo", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("comp-bar", func(c action.Context[any, any]) error {
			return mockCompError
		}),
		rec.newAction("comp-baz", func(c action.Context[any, any]) error {
			return nil
		}),
		saga.Compensate[any, any]("foo", "comp-foo"),
		saga.Compensate[any, any]("bar", "comp-bar"),
		saga.Compensate[any, any]("baz", "comp-baz"),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, mockCompError) {
		t.Errorf("SAGA should fail with %q; got %q", mockCompError, err)
	}

	want := []string{
		"baz", "bar", "foo",
		"comp-bar",
	}
	if !reflect.DeepEqual(rec.chain, want) {
		t.Errorf("Actions should be called in order. want=%s got=%s", want, rec.chain)
	}
}

func TestExecute_compensatorNotFound(t *testing.T) {
	mockError := errors.New("mock error")
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			return mockError
		}),
		saga.Compensate[any, any]("foo", "bar"),
	)

	if err := saga.Execute(context.Background(), s); !errors.Is(err, saga.ErrActionNotFound) {
		t.Errorf("SAGA should fail with %q; got %q", saga.ErrActionNotFound, err)
	}
}

func TestExecute_reportRuntime(t *testing.T) {
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			<-time.After(50 * time.Millisecond)
			return nil
		}),
	)

	var rep report.Report[any, any]
	if err := saga.Execute(context.Background(), s, saga.Report[any, any](&rep)); err != nil {
		t.Fatalf("SAGA shouldn't fail; failed with %q", err)
	}

	if runtime := rep.Runtime; runtime < 50*time.Millisecond || runtime > 100*time.Millisecond {
		t.Errorf("Report should have a runtime of ~50ms; got %s", runtime)
	}
}

func TestExecute_reportError(t *testing.T) {
	var rep report.Report[any, any]
	mockError := errors.New("mock error")
	s := saga.New(
		saga.Action[any, any]("foo", func(action.Context[any, any]) error {
			return mockError
		}),
	)

	if err := saga.Execute(context.Background(), s, saga.Report[any, any](&rep)); !errors.Is(err, mockError) {
		t.Errorf("SAGA should fail with %q; got %q", mockError, err)
	}

	if !errors.Is(rep.Error, mockError) {
		t.Errorf("Report should have error %q; got %q", mockError, rep.Error)
	}
}

func TestExecute_sequence(t *testing.T) {
	rec := newRecorder()
	s := saga.New(
		rec.newAction("foo", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("bar", func(c action.Context[any, any]) error {
			return nil
		}),
		rec.newAction("baz", func(c action.Context[any, any]) error {
			return nil
		}),
		saga.Sequence[any, any]("foo", "baz", "bar"),
	)

	if err := saga.Execute(context.Background(), s); err != nil {
		t.Fatalf("SAGA shouldn't fail; failed with %q", err)
	}

	want := []string{"foo", "baz", "bar"}
	if !reflect.DeepEqual(rec.chain, want) {
		t.Errorf("Actions should be executed in sequence %v; was %v", want, rec.chain)
	}
}

// func TestExecute_eventBus(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	evt := event.New("foo", test.FooEventData{})
// 	s := saga.New(
// 		saga.Action[any, any]("foo", func(c action.Context[any, any]) error {
// 			return c.Publish(c, evt)
// 		}),
// 	)

// 	bus := mock_event.NewMockBus(ctrl)
// 	bus.EXPECT().Publish(gomock.Any(), evt).Return(nil)

// 	if err := saga.Execute(context.Background(), s, saga.EventBus(bus)); err != nil {
// 		t.Errorf("SAGA shouldn't fail; failed with %q", err)
// 	}
// }

// func TestExecute_commandBus(t *testing.T) {
// 	type mockPayload struct{}

// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	cmd := command.New("foo", mockPayload{})
// 	s := saga.New(
// 		saga.Action[any, any]("foo", func(c action.Context[any, any]) error {
// 			return c.Dispatch(c, cmd)
// 		}),
// 	)

// 	bus := mock_command.NewMockBus(ctrl)
// 	bus.EXPECT().Dispatch(gomock.Any(), cmd, gomock.Any()).Return(nil)

// 	if err := saga.Execute(context.Background(), s, saga.CommandBus(bus)); err != nil {
// 		t.Errorf("SAGA shouldn't fail; failed with %q", err)
// 	}
// }

// func TestExecute_repository(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	foo := aggregate.New("foo", uuid.New())
// 	repo := mock_aggregate.NewMockRepository(ctrl)
// 	s := saga.New(
// 		saga.Action[any, any]("foo", func(c action.Context[any, any]) error {
// 			return c.Fetch(c, foo)
// 		}),
// 	)

// 	repo.EXPECT().Fetch(gomock.Any(), foo).Return(nil)

// 	if err := saga.Execute(context.Background(), s, saga.Repository(repo)); err != nil {
// 		t.Fatalf("SAGA shouldn't fail; failed with %q", err)
// 	}
// }

func TestExecute_compensateTimeout(t *testing.T) {
	mockError := errors.New("mock error")
	s := saga.New(
		saga.Action[any, any]("foo", func(c action.Context[any, any]) error {
			return nil
		}),
		saga.Action[any, any]("bar", func(c action.Context[any, any]) error {
			return mockError
		}),
		saga.Action[any, any]("comp-foo", func(action.Context[any, any]) error {
			<-time.After(100 * time.Millisecond)
			return nil
		}),
		saga.Sequence[any, any]("foo", "bar"),
		saga.Compensate[any, any]("foo", "comp-foo"),
	)

	err := saga.Execute(context.Background(), s, saga.CompensateTimeout[any, any](10*time.Millisecond))

	if !errors.Is(err, saga.ErrCompensateTimeout) {
		t.Fatalf("Execute should fail with %q; got %q", saga.ErrCompensateTimeout, err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		opts      []saga.Option[any, any]
		wantError error
	}{
		{
			name: "invalid sequence: action not found",
			opts: []saga.Option[any, any]{
				saga.Action[any, any]("foo", nil),
				saga.Action[any, any]("bar", nil),
				saga.Sequence[any, any]("foo", "bar", "baz"),
			},
			wantError: saga.ErrActionNotFound,
		},
		{
			name: "empty name",
			opts: []saga.Option[any, any]{
				saga.Action[any, any]("   ", nil),
			},
			wantError: saga.ErrEmptyName,
		},
		{
			name: "compensator not found",
			opts: []saga.Option[any, any]{
				saga.Action[any, any]("foo", nil),
				saga.Compensate[any, any]("foo", "bar"),
			},
			wantError: saga.ErrActionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := saga.New(tt.opts...)
			err := saga.Validate(s)

			if !errors.Is(err, tt.wantError) {
				t.Errorf("Validate() should return %q; got %q", tt.wantError, err)
			}
		})
	}
}

func newRecorder() *recorder {
	return &recorder{count: make(map[string]int)}
}

func (r *recorder) newAction(name string, run func(action.Context[any, any]) error) saga.Option[any, any] {
	return saga.Action(name, func(ctx action.Context[any, any]) error {
		defer r.done(name)
		return run(ctx)
	})
}

func (r *recorder) done(name string) {
	r.count[name]++
	r.chain = append(r.chain, name)
}
