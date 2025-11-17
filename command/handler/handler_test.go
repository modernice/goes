package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/handler"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/testutil"
)

func TestBaseHandler_HandleCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a := NewHandlerAggregate(internal.NewUUID())

	cmd := command.NewContext[any](ctx, command.New[any]("foo", "abc"))

	if err := a.HandleCommand(cmd); err != nil {
		t.Fatalf("HandleCommand() failed with %q", err)
	}

	if a.FooVal != "abc" {
		t.Fatalf("FooVal should be %q after %q command; is %q", "abc", "foo", a.FooVal)
	}

	cmd = command.NewContext[any](ctx, command.New[any]("bar", "xyz"))

	if err := a.HandleCommand(cmd); err != nil {
		t.Fatalf("HandleCommand() failed with %q", err)
	}

	if a.BarVal != "xyz" {
		t.Fatalf("BarVal should be %q after %q command; is %q", "xyz", "bar", a.BarVal)
	}
}

func TestOf_Handle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// eventReg := test.NewEncoder()
	eventBus := eventbus.New()
	cmdReg := codec.New()
	codec.Register[string](cmdReg, "foo")
	codec.Register[string](cmdReg, "bar")
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	commandBus := cmdbus.New[int](cmdReg, eventBus)
	repo := repository.New(eventStore)

	h := handler.New(NewHandlerAggregateOpts(), repo, commandBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	id := internal.NewUUID()

	if err := commandBus.Dispatch(ctx, command.New("foo", "abc", command.Aggregate("handler", id)).Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch failed with %q", err)
	}

	if err := commandBus.Dispatch(ctx, command.New("bar", "xyz", command.Aggregate("handler", id)).Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch failed with %q", err)
	}

	foo := NewHandlerAggregate(id)

	if err := repo.Fetch(ctx, foo); err != nil {
		t.Fatalf("Fetch() failed with %q", err)
	}

	if foo.FooVal != "abc" {
		t.Fatalf("FooVal should be %q; is %q", "abc", foo.FooVal)
	}

	if foo.BarVal != "xyz" {
		t.Fatalf("BarVal should be %q; is %q", "xyz", foo.BarVal)
	}
}

func TestBeforeHandle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdReg := codec.New()
	codec.Register[string](cmdReg, "foo")
	eventBus := eventbus.New()
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	commandBus := cmdbus.New[int](cmdReg, eventBus)
	repo := repository.New(eventStore)

	var beforeHandleCalled bool
	h := handler.New(NewHandlerAggregateOpts(handler.BeforeHandle(func(command.Ctx[string]) error {
		beforeHandleCalled = true
		return nil
	}, "foo")), repo, commandBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	cmd := command.New("foo", "bar")
	if err := commandBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if !beforeHandleCalled {
		t.Fatalf("BeforeHandle() callback should have been called")
	}
}

func TestBeforeHandle_wildcard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdReg := codec.New()
	codec.Register[string](cmdReg, "foo")
	eventBus := eventbus.New()
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	commandBus := cmdbus.New[int](cmdReg, eventBus)
	repo := repository.New(eventStore)

	var beforeHandleCalled bool
	h := handler.New(NewHandlerAggregateOpts(handler.BeforeHandle(func(command.Ctx[string]) error {
		beforeHandleCalled = true
		return nil
	})), repo, commandBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	cmd := command.New("foo", "bar")
	if err := commandBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if !beforeHandleCalled {
		t.Fatalf("BeforeHandle() callback should have been called")
	}
}

func TestAfterHandle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdReg := codec.New()
	codec.Register[string](cmdReg, "foo")
	eventBus := eventbus.New()
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	pubBus := cmdbus.New[int](cmdReg, eventBus)
	subBus := cmdbus.New[int](cmdReg, eventBus)
	repo := repository.New(eventStore)

	var afterHandleCalled bool
	h := handler.New(NewHandlerAggregateOpts(handler.AfterHandle(func(command.Ctx[string]) {
		afterHandleCalled = true
	}, "foo")), repo, subBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	cmd := command.New("foo", "bar")
	if err := pubBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if !afterHandleCalled {
		t.Fatalf("AfterHandle() callback should have been called")
	}
}

func TestAfterHandle_wildcard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdReg := codec.New()
	codec.Register[string](cmdReg, "foo")
	eventBus := eventbus.New()
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	commandBus := cmdbus.New[int](cmdReg, eventBus)
	repo := repository.New(eventStore)

	var afterHandleCalled bool
	h := handler.New(NewHandlerAggregateOpts(handler.AfterHandle(func(command.Ctx[string]) {
		afterHandleCalled = true
	})), repo, commandBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	cmd := command.New("foo", "bar")
	if err := commandBus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if !afterHandleCalled {
		t.Fatalf("AfterHandle() callback should have been called")
	}
}

// HandlerAggregate is an aggregate that provides a base implementation for
// handling commands and events. It allows registering handlers for specific
// commands and applying events to update the state of the aggregate.
type HandlerAggregate struct {
	*aggregate.Base
	*handler.BaseHandler

	FooVal string
	BarVal string
}

// NewHandlerAggregateOpts returns a function that creates a new
// HandlerAggregate with the given handler.Options.
func NewHandlerAggregateOpts(opts ...handler.Option) func(uuid.UUID) *HandlerAggregate {
	return func(id uuid.UUID) *HandlerAggregate {
		return NewHandlerAggregate(id, opts...)
	}
}

// NewHandlerAggregate creates a new HandlerAggregate with the given UUID and
// options. The returned HandlerAggregate is capable of handling "foo" and "bar"
// commands, and updating FooVal and BarVal fields upon successful handling of
// these commands.
func NewHandlerAggregate(id uuid.UUID, opts ...handler.Option) *HandlerAggregate {
	a := &HandlerAggregate{
		Base:        aggregate.New("handler", id),
		BaseHandler: handler.NewBase(opts...),
	}

	event.ApplyWith(a, a.foo, "foo")
	event.ApplyWith(a, a.bar, "bar")

	command.HandleWith(a, func(ctx command.Ctx[string]) error {
		return a.Foo(ctx.Payload())
	}, "foo")

	command.HandleWith(a, func(ctx command.Ctx[string]) error {
		return a.Bar(ctx.Payload())
	}, "bar")

	return a
}

// Foo applies the given input string to the HandlerAggregate by generating a
// "foo" event with the input data, calling aggregate.Next, and updating FooVal
// with the input value.
func (a *HandlerAggregate) Foo(input string) error {
	aggregate.Next(a, "foo", test.FooEventData{A: input})
	return nil
}

func (a *HandlerAggregate) foo(evt event.Of[test.FooEventData]) {
	a.FooVal = evt.Data().A
}

// Bar updates the BarVal field of HandlerAggregate by generating a "bar" event
// with the given input data, calling aggregate.Next, and updating BarVal with
// the input value.
func (a *HandlerAggregate) Bar(input string) error {
	aggregate.Next(a, "bar", test.BarEventData{A: input})
	return nil
}

func (a *HandlerAggregate) bar(evt event.Of[test.BarEventData]) {
	a.BarVal = evt.Data().A
}
