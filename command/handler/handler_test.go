package handler_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/encoding"
	"github.com/modernice/goes/command/handler"
	mock_command "github.com/modernice/goes/command/mocks"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/internal/errbus"
)

type recorder struct {
	h      func(context.Context, command.Command) error
	params chan handlerParams
}

type handlerParams struct {
	ctx context.Context
	cmd command.Command
}

type mockPayload struct{}

type mockBus struct {
	errs *errbus.Bus
}

func TestHandler_On(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)
	h := handler.New(bus)

	rec := newRecorder(nil)
	h.On(context.Background(), "foo", rec.Handle)

	cmd := command.New("foo", mockPayload{})
	if err := bus.Dispatch(context.Background(), cmd); err != nil {
		t.Fatalf("failed to dispatch Command: %v", err)
	}

	timeout, stop := after(3 * time.Second)
	defer stop()

	select {
	case <-timeout:
		t.Fatalf("didn't receive Command after %s", time.Second)
	case p := <-rec.params:
		if p.ctx == nil {
			t.Errorf("handler received <nil> Context!")
		}

		if !reflect.DeepEqual(p.cmd, cmd) {
			t.Errorf("handler received wrong Command. want=%v got=%v", cmd, p.cmd)
		}
	}
}

func TestHandler_On_cancelContext(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)
	h := handler.New(bus)

	rec := newRecorder(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.On(ctx, "foo", rec.Handle)
	cancel()

	<-time.After(10 * time.Millisecond)

	cmd := command.New("foo", mockPayload{})
	if err := bus.Dispatch(context.Background(), cmd); err == nil {
		t.Fatal("dispatch should have failed, but didn't!")
	}

	timeout, stop := after(10 * time.Millisecond)
	defer stop()

	select {
	case <-rec.params:
		t.Fatalf("handler should not have received a Command!")
	case <-timeout:
	}
}

func TestHandler_On_busError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bus := mock_command.NewMockBus(ctrl)
	h := handler.New(bus)

	mockError := errors.New("mock error")
	bus.EXPECT().Subscribe(gomock.Any(), "foo").Return(nil, mockError)

	rec := newRecorder(nil)

	if err := h.On(context.Background(), "foo", rec.Handle); !errors.Is(err, mockError) {
		t.Errorf("On should fail with %q; got %q", mockError, err)
	}
}

func TestHandler_busError(t *testing.T) {
	bus := newMockBus()
	h := handler.New(bus)

	errs := h.Errors(context.Background())

	<-time.After(50 * time.Millisecond)

	mockError := errors.New("mock error")
	bus.errs.Publish(context.Background(), mockError)

	timeout, stop := after(100 * time.Millisecond)
	defer stop()

	select {
	case <-timeout:
		t.Fatalf("didn't receive error after %s", 100*time.Millisecond)
	case err := <-errs:
		if err != mockError {
			t.Errorf("received wrong error. want=%q got=%q", mockError, err)
		}
	}
}

func TestHandler_done(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", mockPayload{})
	ebus := chanbus.New()
	bus := cmdbus.New(enc, ebus)
	h := handler.New(bus)

	events, err := ebus.Subscribe(context.Background(), cmdbus.CommandExecuted)
	if err != nil {
		t.Fatalf("failed to subscribe to %q events: %v", "foo", err)
	}

	h.On(context.Background(), "foo", func(context.Context, command.Command) error {
		return nil
	})

	cmd := command.New("foo", mockPayload{})
	if err := bus.Dispatch(context.Background(), cmd); err != nil {
		t.Fatalf("failed to dispatch Command: %v", err)
	}

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

L:
	for {
		select {
		case <-timeout.C:
			t.Fatalf("didn't receive Event after %s", 3*time.Second)
		case <-events:
			break L
		}
	}
}

func newRecorder(h func(context.Context, command.Command) error) *recorder {
	if h == nil {
		h = func(context.Context, command.Command) error { return nil }
	}
	return &recorder{
		h:      h,
		params: make(chan handlerParams, 1),
	}
}

func (r *recorder) Handle(ctx context.Context, cmd command.Command) error {
	r.params <- handlerParams{ctx, cmd}
	return r.h(ctx, cmd)
}

func after(d time.Duration) (<-chan time.Time, func()) {
	timer := time.NewTimer(d)
	return timer.C, func() {
		timer.Stop()
	}
}

func newMockBus() *mockBus {
	return &mockBus{
		errs: errbus.New(),
	}
}

func (b *mockBus) Dispatch(context.Context, command.Command, ...dispatch.Option) error { return nil }

func (b *mockBus) Subscribe(context.Context, ...string) (<-chan command.Context, error) {
	return nil, nil
}

func (b *mockBus) Errors(ctx context.Context) <-chan error {
	return b.errs.Subscribe(ctx)
}
