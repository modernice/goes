# SAGAs / Process Managers

The `saga` package implements a SAGA coordinator / process manager for more
complex multi-step transactions. This package integrates with the event,
aggregate and command system to provide convenient access to the different
components within the defined SAGA actions.

This SAGA implementation is very likely subject to a rewrite. This is due to the
implementation not working distributed like the [command bus](
http://github.com/modernice/goes/command) or [projection service](
http://github.com/modernice/goes/projection). Instead, a SAGA is executed and
coordinates from within a single process, which provides no recover strategy for
when the process that executes the SAGA fails. A rewrite is necessary to make
SAGAs work event-driven, which should provide the required persistence layer and
state needed for recovering SAGAs.

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
