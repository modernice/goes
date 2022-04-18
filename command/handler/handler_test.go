package handler_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/handler"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/testutil"
)

func TestBaseHandler_HandleCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := NewHandlerAggregate(uuid.New())

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventReg := test.NewEncoder()
	eventBus := eventbus.New()
	eventStore := eventstore.WithBus(eventstore.New(), eventBus)
	commandBus := cmdbus.New(eventReg, eventBus)
	repo := repository.New(eventStore)

	h := handler.New(NewHandlerAggregate, repo, commandBus)

	errs, err := h.Handle(ctx)
	if err != nil {
		t.Fatalf("Handle() failed with %q", err)
	}
	go testutil.PanicOn(errs)

	id := uuid.New()

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

type HandlerAggregate struct {
	*aggregate.Base
	*handler.BaseHandler

	FooVal string
	BarVal string
}

func NewHandlerAggregate(id uuid.UUID) *HandlerAggregate {
	a := &HandlerAggregate{
		Base:        aggregate.New("handler", id),
		BaseHandler: handler.NewBase(),
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

func (a *HandlerAggregate) Foo(input string) error {
	aggregate.Next(a, "foo", test.FooEventData{A: input})
	return nil
}

func (a *HandlerAggregate) foo(evt event.Of[test.FooEventData]) {
	a.FooVal = evt.Data().A
}

func (a *HandlerAggregate) Bar(input string) error {
	aggregate.Next(a, "bar", test.BarEventData{A: input})
	return nil
}

func (a *HandlerAggregate) bar(evt event.Of[test.BarEventData]) {
	a.BarVal = evt.Data().A
}
