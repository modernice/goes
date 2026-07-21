package action_test

import (
	"context"
	"testing"

	"github.com/modernice/goes/saga/action"
)

type mockPayload struct{}

func TestContext(t *testing.T) {
	var ctx action.Context
	var _ context.Context = ctx
}

func TestContext_Current(t *testing.T) {
	parent := context.Background()
	act := action.New("foo", func(c action.Context) error { return nil })
	ctx := action.NewContext(parent, act)

	if ctx.Action() != act {
		t.Errorf("Current() should return %v; got %v", act, ctx.Action())
	}
}

// func TestContext_Publish(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	ctx := action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 	)

// 	evt := event.New("foo", test.FooEventData{})
// 	if err := ctx.Publish(context.Background(), evt.Any()); !errors.Is(err, action.ErrMissingBus) {
// 		t.Errorf("Publish() should fail with %q; got %q", action.ErrMissingBus, err)
// 	}

// 	bus := mock_event.NewMockBus(ctrl)
// 	ctx = action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 		action.WithEventBus(bus),
// 	)

// 	bus.EXPECT().Publish(gomock.Any(), evt).Return(nil)

// 	if err := ctx.Publish(context.Background(), evt); err != nil {
// 		t.Errorf("Publish() should't fail; failed with %q", err)
// 	}

// 	bus = mock_event.NewMockBus(ctrl)
// 	ctx = action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 		action.WithEventBus(bus),
// 	)

// 	mockError := errors.New("mock error")
// 	bus.EXPECT().Publish(gomock.Any(), evt).Return(mockError)

// 	if err := ctx.Publish(context.Background(), evt); !errors.Is(err, mockError) {
// 		t.Errorf("Publish() should fail with %q; got %q", mockError, err)
// 	}
// }

// func TestContext_Dispatch(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	ctx := action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 	)

// 	cmd := command.New("foo", mockPayload{})
// 	if err := ctx.Dispatch(context.Background(), cmd); !errors.Is(err, action.ErrMissingBus) {
// 		t.Errorf("Dispatch() should fail with %q; got %q", action.ErrMissingBus, err)
// 	}

// 	bus := mock_command.NewMockBus(ctrl)
// 	ctx = action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 		action.WithCommandBus(bus),
// 	)

// 	var dispatchCfg command.DispatchConfig
// 	bus.EXPECT().
// 		Dispatch(gomock.Any(), cmd, gomock.Any()).
// 		DoAndReturn(func(_ context.Context, _ command.Command, opts ...command.DispatchOption) error {
// 			dispatchCfg = dispatch.Configure(opts...)
// 			return nil
// 		})

// 	if err := ctx.Dispatch(context.Background(), cmd); err != nil {
// 		t.Errorf("Dispatch() should't fail; failed with %q", err)
// 	}

// 	bus = mock_command.NewMockBus(ctrl)
// 	ctx = action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(c action.Context[E, C]) error { return nil }),
// 		action.WithCommandBus(bus),
// 	)

// 	mockError := errors.New("mock error")
// 	bus.EXPECT().Dispatch(gomock.Any(), cmd, gomock.Any()).Return(mockError)

// 	if err := ctx.Dispatch(context.Background(), cmd); !errors.Is(err, mockError) {
// 		t.Errorf("Dispatch() should fail with %q; got %q", mockError, err)
// 	}

// 	if !dispatchCfg.Synchronous {
// 		t.Errorf("Dispatches should always be synchronous!")
// 	}
// }

func TestContext_Run(t *testing.T) {
	ctx := action.NewContext(
		context.Background(),
		action.New("foo", func(c action.Context) error { return nil }),
	)

	if err := ctx.Run(context.Background(), "bar"); err != nil {
		t.Errorf("Run() should be a no-op if no runner is configured.")
	}

}

func TestContext_Run_customRunner(t *testing.T) {
	var called bool
	var calledName string
	ctx := action.NewContext(
		context.Background(),
		action.New("foo", func(c action.Context) error { return nil }),
		action.WithRunner(func(_ context.Context, name string) error {
			called = true
			calledName = name
			return nil
		}),
	)

	if err := ctx.Run(context.Background(), "bar"); err != nil {
		t.Errorf("Run(%q) shouldn't fail; failed with %q", "bar", err)
	}

	if !called {
		t.Errorf("runner wasn't called")
	}

	if calledName != "bar" {
		t.Errorf("runner called with wrong action name. want=%s got=%s", "bar", calledName)
	}
}

// func TestContext_Fetch(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	repo := mock_aggregate.NewMockRepository(ctrl)
// 	mockError := errors.New("mock error")

// 	ctx := action.NewContext(
// 		context.Background(),
// 		action.New("foo", func(ctx action.Context[E, C]) error { return nil }),
// 		action.WithRepository(repo),
// 	)

// 	repo.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(mockError)

// 	foo := aggregate.New("foo", internal.NewUUID())
// 	if err := ctx.Fetch(ctx, foo); !errors.Is(err, mockError) {
// 		t.Fatalf("Fetch should fail with %q; got %q", mockError, err)
// 	}
// }
