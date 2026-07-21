# SAGAs / Process Managers

> **Deprecated:** This package has been superseded by the
> [`workflow`](../workflow) package and will be removed in a future release.
> Use `workflow` for all new code.

The `saga` package implements an **in-process** SAGA coordinator: it runs a
predefined sequence of actions within a single process and compensates
completed actions in reverse order when a later action fails. Because
execution is neither persisted nor distributed, a crashed process cannot
recover a running SAGA — the limitation that motivated the replacement.

The [`workflow`](../workflow) package replaces it with a durable, event-driven
workflow runtime: workflow instances are event-sourced aggregates, every step
is persisted, commands and timeouts are recorded as durable effects, and any
service instance can recover and resume the work of a crashed one.

## Migrating to `workflow`

| `saga` | `workflow` |
| --- | --- |
| `saga.New(...)` (a `saga.Setup`) | `workflow.Define(NewOrderWorkflow, ...)` |
| `saga.Action("step", fn)` | Event-driven handlers: `workflow.Starts(...)`, `workflow.Reacts(...)` |
| `saga.Sequence(...)`, `saga.StartWith(...)` | No fixed sequence — steps advance by reacting to domain events |
| `saga.Compensate("step", "undo-step")` | Explicit compensation phase: `ctx.Compensate(err)`, `workflow.Compensates(...)` handlers, `ctx.Compensated()` |
| `saga.NewExecutor(...).Execute(ctx, setup)` | `workflow.NewService(workflow.Config{...}, defs...)` + `svc.Run(ctx)` |
| `saga.Repository()`, `saga.EventBus()`, `saga.CommandBus()` | `workflow.Config{EventStore, EventBus, CommandBus, Commands}` |
| `action.Context.Dispatch(...)` | `ctx.Dispatch("effect-key", cmd)` — a durable, at-least-once command effect |
| `action.Context.Publish(...)` | Workflows react to events published by your aggregates and command handlers |
| `action.Context.Fetch(...)` | The workflow instance itself is event-sourced state; fetch other aggregates in your command handlers |
| `saga.Report()` / `report.Report` | `Status()` / `Reason()` on the workflow, plus the built-in `goes.workflow.*` events |
| — | Durable timeouts: `ctx.Schedule(...)`, `workflow.OnTimeout(...)` |

The semantics change with the migration:

- A SAGA executes once, synchronously, in the calling process. A workflow is a
  long-lived, durable instance that is advanced by events — across service
  restarts and across multiple service instances.
- Control flow is inverted: instead of a central sequence invoking actions,
  each handler dispatches commands whose resulting domain events trigger the
  next handler.
- Compensation is not automatic: entering compensation is an explicit decision
  (`ctx.Compensate`), the undo work is driven by `Compensates` handlers
  reacting to events, and `ctx.Compensated()` (or `ctx.CompensationFailed()`)
  ends the phase.

See the [workflow README](../workflow/README.md) for the full model, delivery
semantics, and a complete example.

---

# Legacy documentation

## Features

- Action-based SAGA setups
- Compensators
- Flexible action invoking
- Integrates with goes' components

## Setups

The main type of the SAGA implementation is the `saga.Setup` interface, which
provides the executor with runnable actions and some configuration:

```go
package saga

type Setup interface {
  // Sequence returns the names of the actions that should be run sequentially.
  Sequence() []string

  // Compensator finds and returns the name of the compensating action for the
  // Action with the given name. If Compensator returns an empty string, there
  // is no compensator for the given action configured.
  Compensator(string) string

  // Action returns the action with the given name. Action returns nil if no
  // Action with that name was configured.
  Action(string) action.Action
}
```

### Actions

A `saga.Setup` implementation can be instantiated using `saga.New()`. Pass
`saga.Action()` options to define the actions within the SAGA.

```go
package example

import (
  "github.com/modernice/goes/saga"
  "github.com/modernice/goes/saga/action"
)

func example() {
  setup := saga.New(
    saga.Action("foo", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("bar", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("baz", func(ctx action.Context) error {
       return nil
    }),
  )
}
```

### Sequence / Starting Action

The `saga.Sequece()` option defines the order in which the actions of the SAGA
are run. When an action returns a non-nil error, remaining actions are not run.

```go
package example

import (
  "github.com/modernice/goes/saga"
  "github.com/modernice/goes/saga/action"
)

func example() {
  setup := saga.New(
    saga.Action("foo", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("bar", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("baz", func(ctx action.Context) error {
       return nil
    }),

    // Run "bar", then "baz", finally "foo".
    saga.Sequence("bar", "baz", "foo"),
  )
}
```

Alternatively, the `saga.StartWith()` option can be used to specify a single
action that should be run when executing this setup. Using the action's context,
other actions can be called from within a running action by name.

```go
package example

import (
  "github.com/modernice/goes/saga"
  "github.com/modernice/goes/saga/action"
)

func example() {
  setup := saga.New(
    saga.Action("foo", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("bar", func(ctx action.Context) error {
      return ctx.Run(ctx, "baz")
    }),
    saga.Action("baz", func(ctx action.Context) error {
      return ctx.Run(ctx, "foo")
    }),

    // Same as saga.Sequece("bar")
    saga.StartWith("bar"),
  )
}
```

If neither the `saga.Sequence()` nor the saga `saga.StartWith()` option is
passed, the first defined action is automatically used as the starting action to
the SAGA (effectively an automatic `saga.StartWith()` option using the first
defined action).


### Compensating Actions

Compensating actions are run to gracefully recover or fix application state when
the execution of a setup fails. When an action of a SAGA fails, all previously
run actions that returned **no error** will be compensating by running their
configured compensator action (if any), in reverse order. Any action can be
configured as the compensating action for another action.

```go
package example

import (
  "github.com/modernice/goes/saga"
  "github.com/modernice/goes/saga/action"
)

func example() {
  setup := saga.New(
    saga.Action("foo", func(ctx action.Context) error {
       return nil
    }),
    saga.Action("bar", func(ctx action.Context) error {
      return nil
    }),
    saga.Action("baz", func(ctx action.Context) error {
      return errors.New("oops")
    }),

    saga.Action("fix-foo", func(ctx action.Context) error {
      return nil
    }),

    saga.Action("fix-bar", func(ctx action.Context) error {
      return nil
    }),

    saga.Sequence("foo", "bar", "baz"),
    saga.Compensate("foo", "fix-foo"),
    saga.Compensate("bar", "fix-bar"),
  )
}
```

The example above would result the following actions be run in the given order:

- "foo"
- "bar"
- "baz"
- "fix-bar"
- "fix-foo"

### Action Context

The `action.Context` that is passed to action functions provides functions to
fetch aggregates, publish events and dispatch commands. For this to work, the
`saga.Executor` that executes the setup needs to have the aggregate repository,
event bus and/or command bus explicitly configured (see [Executor Options](
#executor-options)); otherwise these following functions return
`action.ErrMissingRepository` errors.

```go
package example

func example() {
  setup := saga.New(
    saga.Action("foo", func(ctx action.Context) error {
      err := ctx.Fetch(ctx, ...)
      err := ctx.Publish(ctx, event.New(...))
      err := ctx.Dispatch(ctx, command.New(...))
    }),
  )
}
```

## Executor

A SAGA setup can be executed using a `saga.Executor`.

```go
package example

func example(setup saga.Setup) {
  exec := saga.NewExecutor()
  err := exec.Execute(context.TODO(), setup)
}
```

### Executor Options

When creating an executor, use the

- `saga.Repository()` ,
- `saga.EventBus()` and
- `saga.CommandBus()`

options to provide the executor with repositories which will be available from within the SAGAs `action.Context`s.
