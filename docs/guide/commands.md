# Commands

Commands express intent — they are requests to do something. A command is dispatched through a bus and handled by exactly one handler. The command bus is primarily useful in distributed systems where the dispatch site and the handler run in separate processes or services. In a single-process application, you can call aggregate methods directly (via `repo.Use`) without the overhead of a command bus.

> For a step-by-step introduction, see the [Tutorial](/tutorial/06-commands).

## Defining Commands

Like events, commands have a name and a payload type:

```go
const CreateProductCmd = "shop.product.create"

type CreateProductPayload struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
	Stock int    `json:"stock"`
}
```

## Creating Commands

```go
cmd := command.New(CreateProductCmd, CreateProductPayload{
	Name:  "Wireless Mouse",
	Price: 2999,
	Stock: 50,
}, command.Aggregate(ProductAggregate, productID))
```

The `command.Aggregate` option links the command to a specific aggregate instance. This lets the handler know which aggregate to load.

### Options

| Option | Description |
| --- | --- |
| `command.ID(uuid)` | Override the command ID |
| `command.Aggregate(name, id)` | Link the command to an aggregate |

## Command Bus

The command bus dispatches commands to handlers. Because it uses the event bus as transport, commands dispatched in one process can be handled by another.

The `command.Bus` interface:

```go
type Bus interface {
	Dispatch(ctx context.Context, cmd Command, opts ...DispatchOption) error
	Subscribe(ctx context.Context, names ...string) (<-chan Ctx[any], <-chan error, error)
}
```

The implementation uses the event bus for transport:

```go
import "github.com/modernice/goes/command/cmdbus"

cbus := cmdbus.New[int](cmdReg, eventBus)
```

The type parameter (`int` above) is the error code type for execution errors. The `cmdReg` argument is a `codec.Encoding` — a separate [codec registry](/guide/codec) for command payloads. In practice, you should create two registries: one for events and one for commands. Using a single registry is possible if no event name collides with a command name, but keeping them separate is cleaner.

### Command Bus Options

| Option | Default | Description |
| --- | --- | --- |
| `AssignTimeout(d)` | `5s` | Max time to wait for a handler to subscribe |
| `ReceiveTimeout(d)` | `10s` | Max time for the handler to process the command |
| `Workers(n)` | — | Number of concurrent command processors |
| `Filter(fn)` | — | Filter which commands to accept |
| `Debug(bool)` | `false` | Enable debug logging |

Set a timeout to `0` to disable it.

`AssignTimeout` controls how long `Dispatch` waits for a handler to be registered for the command. If no handler subscribes within this duration, dispatch returns `ErrAssignTimeout`.

`ReceiveTimeout` controls how long the bus waits for the handler to acknowledge receipt. If the handler doesn't pick up the command in time, dispatch returns `ErrReceiveTimeout`.

## Handling Commands

`command.MustHandle` subscribes a handler for a command name and panics if subscription fails. `command.Handle` is the non-panicking variant that returns the subscription error:

```go
errs := command.MustHandle(ctx, cbus, CreateProductCmd, handler)
errs, err := command.Handle(ctx, cbus, CreateProductCmd, handler)
```

Both return an error channel for asynchronous handler errors. `MustHandle` is the right choice most of the time — command subscriptions typically happen at service startup, and a failed subscription means the service cannot function correctly. Panicking early surfaces the problem immediately.

```go
errs := command.MustHandle(ctx, cbus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
	payload := ctx.Payload()
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		return p.Create(payload.Name, payload.Price, payload.Stock)
	})
})
```

### Command Context

The handler receives a `command.Ctx[P]` which provides:

| Method | Description |
| --- | --- |
| `Payload()` | The typed command payload |
| `AggregateID()` | The linked aggregate's UUID |
| `AggregateName()` | The linked aggregate's name |

`command.Ctx[P]` embeds `context.Context`, so it can be passed to repository methods directly.

## Aggregate-Owned Command Handlers

`command.MustHandle(...)` is the best default when command handling lives in application setup code:

- a handler orchestrates multiple aggregates
- a handler calls other services or clients
- the command flow is more workflow than aggregate API

Sometimes the command handling belongs inside the aggregate itself. In that case, use `command/handler`.

The pattern has two parts:

1. The aggregate registers its own command handlers.
2. `command/handler.New(...)` subscribes the bus and routes matching commands into that aggregate.

### Example

```go
package todo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	cmdhandler "github.com/modernice/goes/command/handler"
	"github.com/modernice/goes/event"
)

const (
	ListAggregate = "todo.list"

	AddTaskCmd    = "todo.list.add_task"
	RemoveTaskCmd = "todo.list.remove_task"

	TaskAdded   = "todo.list.task_added"
	TaskRemoved = "todo.list.task_removed"
)

type List struct {
	*aggregate.Base
	*cmdhandler.BaseHandler
	Tasks []string
}

func NewList(id uuid.UUID) *List {
	list := &List{
		Base: aggregate.New(ListAggregate, id),
		BaseHandler: cmdhandler.NewBase(
			cmdhandler.BeforeHandle(func(ctx command.Ctx[string]) error {
				if ctx.Payload() == "" {
					return errors.New("task must not be empty")
				}
				return nil
			}, AddTaskCmd),
		),
		Tasks: make([]string, 0),
	}

	event.ApplyWith(list, list.taskAdded, TaskAdded)
	event.ApplyWith(list, list.taskRemoved, TaskRemoved)

	command.HandleWith(list, func(ctx command.Ctx[string]) error {
		return list.AddTask(ctx.Payload())
	}, AddTaskCmd)

	command.HandleWith(list, func(ctx command.Ctx[string]) error {
		return list.RemoveTask(ctx.Payload())
	}, RemoveTaskCmd)

	return list
}

func (l *List) AddTask(task string) error {
	aggregate.Next(l, TaskAdded, task)
	return nil
}

func (l *List) RemoveTask(task string) error {
	aggregate.Next(l, TaskRemoved, task)
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

func run(ctx context.Context, cbus command.Bus, repo aggregate.Repository) <-chan error {
	return cmdhandler.New(NewList, repo, cbus).MustHandle(ctx)
}
```

### `command/handler.Base`

The core type is `command/handler.Base`. In aggregates you will usually embed the `BaseHandler` alias from the same package because it avoids colliding with `*aggregate.Base`.

`NewBase(...)` sets up the command registration surface:

- `RegisterCommandHandler(...)`
- `CommandNames()`
- `HandleCommand(...)`

`command.HandleWith(...)` is the usual way to register typed command handlers on that base.

### `BeforeHandle` and `AfterHandle`

`BeforeHandle(...)` and `AfterHandle(...)` attach hooks around aggregate-owned command handling.

Use `BeforeHandle(...)` for cheap validation or guard rails:

```go
cmdhandler.NewBase(
	cmdhandler.BeforeHandle(func(ctx command.Ctx[string]) error {
		if ctx.Payload() == "" {
			return errors.New("task must not be empty")
		}
		return nil
	}, AddTaskCmd),
)
```

Use `AfterHandle(...)` for post-success side effects like metrics or logs:

```go
cmdhandler.NewBase(
	cmdhandler.AfterHandle(func(ctx command.Ctx[string]) {
		log.Printf("handled %s", ctx.Name())
	}),
)
```

If you omit command names, the hook applies to all commands handled by the aggregate.

::: tip
`command/handler.New(...)` instantiates the aggregate once during setup to discover its registered command names. Keep aggregate constructors side-effect free.
:::

### Which style should you choose?

Use free-standing `command.MustHandle(...)` when:

- the handler coordinates multiple aggregates
- the handler talks to external systems
- the command flow is easier to read in application setup

Use `command/handler.New(...)` when:

- the command maps cleanly to one aggregate
- you want the aggregate to declare its own command surface
- you want reusable per-aggregate hooks with `BeforeHandle(...)` and `AfterHandle(...)`

## Synchronous Dispatch

By default, `Dispatch` is asynchronous — it returns once a handler has received the command, but without waiting for the handler to finish processing it. This still guarantees delivery: if no handler picks up the command within the `AssignTimeout` / `ReceiveTimeout`, `Dispatch` returns an error.

For synchronous dispatch, use `dispatch.Sync()`:

```go
import "github.com/modernice/goes/command/cmdbus/dispatch"

err := cbus.Dispatch(ctx, cmd.Any(), dispatch.Sync())
// err contains the handler's error, if any.
```

With `dispatch.Sync()`, `Dispatch` blocks until the handler finishes and returns the handler's error directly.

## When to Use a Workflow

Commands and workflows solve different problems:

- A command is a single intent message.
- A command handler is an immediate request handler.
- A [workflow](/guide/workflows) coordinates a multi-step process that reacts to later events, sets timeouts, and can enter compensation.

If the process needs to wait for future events, survive across restarts, cross service/process boundaries, or manage explicit timeouts and compensation, reach for a workflow rather than stretching a single command handler across the whole process.

## The Command Pattern

The typical flow:

1. Define command names and payload types
2. Register payloads in the codec
3. Subscribe handlers that use `repo.Use` to load, modify, and save aggregates
4. Dispatch commands from your API layer

```go
// In your setup:
errs := command.MustHandle(ctx, cbus, CreateProductCmd, func(ctx command.Ctx[CreateProductPayload]) error {
	return products.Use(ctx, ctx.AggregateID(), func(p *Product) error {
		pl := ctx.Payload()
		return p.Create(pl.Name, pl.Price, pl.Stock)
	})
})
go func() {
	for err := range errs {
		log.Printf("command handler error: %v", err)
	}
}()

// In your API handler:
cmd := command.New(CreateProductCmd, CreateProductPayload{
	Name:  "Wireless Mouse",
	Price: 2999,
	Stock: 50,
}, command.Aggregate(ProductAggregate, uuid.New()))

err := cbus.Dispatch(ctx, cmd.Any(), dispatch.Sync())
```
