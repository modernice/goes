# Commands

Package `command` defines and implements a command system for distributed
applications. Communication between processes is accomplished using the
underlying [event system](../event). Read the [aggregate documentation](
../aggregate) before reading further.

> The command system does not add any benefits to an application that does not
consist of multiple services that need to dispatch commands to each other.
Applications that do not need to dispatch commands to other services should
call the "commands" directly on the aggregates instead.

## Introduction

A command bus can dispatch and subscribe to commands:

```go
package command

type Bus interface {
	Dispatch(context.Context, Command, ...DispatchOption) error

	Subscribe(ctx context.Context, names ...string) (
		<-chan Context,
		<-chan error,
		error,
	)
}
```

The `cmdbus` package provides the event-driven implementation of the command
bus. Use the `cmdbus.New` constructor to create a command bus from an event bus:

```go
package example

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/command/cmdbus"
)

func example(ebus event.Bus) {
	bus := cmdbus.New(ebus)
}
```

Read the documentation of `Bus` for information on how to use it.

### Dispatch a command

Use the `Bus.Dispatch()` method to dispatch a command to a subscribed bus.
Optionally provide the `dispatch.Sync()` option to synchronously dispatch the
command (wait for the execution of the command before returning).

```go
package example

import (
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/dispatch"
)

func example(bus command.Bus) {
	cmd := command.New("foo", <some-payload>, <options>...).Any()

	if err := bus.Dispatch(context.TODO(), cmd); err != nil {
		panic(fmt.Errof("dispatch command: %w", err))
	}

	if err := bus.Dispatch(context.TODO(), cmd, dispatch.Sync()); err != nil {
		panic(fmt.Errof("dispatch and execute command: %w", err))
	}
}
```

### Subscribe to commands

Use the `Bus.Subscribe()` method to subscribe to commands.

```go
package example

import (
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/streams"
)

func example(bus command.Bus) {
	commands, errs, err := bus.Subscribe(context.TODO(), "foo", "bar", "baz")
	if err != nil {
		panic(fmt.Errorf("subscribe to commands: %w", err))
	}

	streams.ForEach(
		context.TODO(),
		func(ctx command.Context) {
			defer ctx.Finish(ctx)
			log.Printf("Received %q command.", ctx.Name())
		},
		func(err error) { log.Println(err) },
		commands,
		errs,
	)
}
```

## Command handling

### Standalone command handler

The command bus provides the low-level API for command communication between
services. For the actual implementation of command handlers, this package
provides a `*Handler` type that wraps a `Bus` to allow for a convenient setup
of command handlers. `*Handler` also automatically calls `ctx.Finish()` after
handling the command.

```go
package example

func example(bus command.Bus) {
	h := command.NewHandler(bus)

	errs, err := h.Handle(
		context.TODO(),
		"foo",
		func(ctx command.Context) error {
			log.Printf("Handling %q command ...", ctx.Name())
			return nil
		},
	)
	// handle err

	for err := range errs {
		log.Printf("failed to handle %q command: %v", "foo", err)
	}
}
```

Using a `Bus` to handle commands is quite l

### Aggregate-based command handler

For commands that act on aggregates, you can use the command handler provided by
the `handler` package for a convenient command setup. Using this handler, you
can to do the following:

```go
package todo

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/handler"
	"github.com/modernice/goes/event"
)

type List struct {
	*aggregate.Base
	*handler.BaseHandler

	Tasks []string
}

func NewList(id uuid.UUID) *List {
	list := &List{
		Base: 			 aggregate.New("list", id),
		BaseHandler: handler.NewBase(),
	}

	event.ApplyWith(list, list.taskAdded, "task_added")
	event.ApplyWith(list, list.taskRemoved, "task_removed")

	command.ApplyWith(list, list.AddTask, "add_task")
	command.ApplyWith(list, list.RemoveTask, "remove_task")

	return list
}

func (l *List) AddTask(task string) error {
	aggregate.Next(l, "task_added", task)
	return nil
}

func (l *List) RemoveTask(task string) error {
	aggregate.Next(l, "task_removed", task)
	return nil
}

func (l *List) taskAdded(evt event.Of[string]) {
	l.Tasks = append(l.Tasks, evt.Data())
}

func (l *List) taskRemoved(evt event.Of[string]) {
	for i, task := range l.Tasks {
		if task == evt.Data() {
			l.Tasks = append(l.Tasks[:i], l.Tasks[i+1:]...)
			return
		}
	}
}

func example(bus command.Bus, repo aggregate.Repository) {
	h := handler.New(NewList, repo, bus)

	errs, err := h.Handle(context.TODO())
	// handle err

	for err := range errs {
		log.Printf("failed to handle %q command: %v", "list", err)
	}
}
```

## Things to consider

### Load-balancing

Do not provide a load-balanced event bus as the underlying event bus for the
command bus implemented by the `cmdbus` package. This will result in broken
communication between the command buses of the different services / service
instances.

### Long-running commands

Handling of commands is done synchronously for each received command within
the standalone `*Handler` and aggregate-based `*handler.Of` command handlers.
This means that while a command is handled, the command handler does not receive
from the underlying event bus, which may cause the event bus to drop events,
depending on the implementation. For example, the NATS event bus has a
`PullTimeout()` option that specifies the timeout after which an event is
dropped if it's not received.

For long-running commands, consider pushing the commands into a queue to avoid
event losses:

```go
package example

func example(bus command.Bus) {
	h := command.NewHandler(bus)

	queue := make(chan command.Context)

	enqueueErrors := h.MustHandle(context.TODO(), "foo", func(ctx command.Context) error {
		go func(){
			select {
			case <-ctx.Done():
			case queue <- ctx:
			}
		}()
		return nil
	})

	cmdErrors := make(chan error)

	go func(){
		defer close(cmdErrors)
		for {
			select {
			case <-ctx.Done():
				return
			case ctx := <-queue:
				var err error // handler error
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case cmdErrors <- err:
					}
				}
			}
		}
	}()

	for err := range cmdErrors {
		log.Printf("failed to handle %q command: %v", "foo", err)
	}
}
```
