# Command System

goes provides a distributed, event-driven command system for inter-process
command handling. The command system communicates and coordinates between
different services/processes to ensure that a command is sent to the appropriate
handler.

The command system is only needed in applications that consist of multiple
(micro-)services that need to dispatch commands to one another over a network.


## Setup

The command system communicates between processes using the [Event System](
http://github.com/modernice/goes/tree/main/event). Use the `cmdbus.New` constructor to
create the event-driven command bus.

```go
package example

import (
  "github.com/modernice/goes/tree/main/codec"
  "github.com/modernice/goes/tree/main/command/cmdbus"
  "github.com/modernice/goes/tree/main/event/eventbus"
)

func example() {
  ereg := codec.New() // Event registry
  creg := codec.New() // Command registry
  ebus := eventbus.New() // In-memory event bus

  cbus := cmdbus.New(creg, ereg, ebus)

  // Subscribe to commands
  commands, errs, err := cbus.Subscribe(context.TODO(), ...)

  // Dispatch commands
  err := cbus.Dispatch(context.TODO(), ...)
}
```

## Define Commands

Just like events, commands need to be defined and most likely to be registered
into a command registry for encoding/decoding.

```go
// Package auth is an example authentication service.
package auth

const UserAggregate = "auth.user"

type User struct { ... } // User aggregate

// Events
const (
  UserRegistered = "auth.user.registered"
)

type UserRegisteredData struct { ... }

func (u *User) Register(name, email string) error {
  // Implementation ...
  return nil
}

// Commands
const (
  RegisterUserCmd = "auth.user.register"
)

type registerUserPayload struct {
  Name  string
  Email string
}

// RegisterUser returns the command to register a user with the given name and
// email address.
func RegisterUser(id uuid.UUID, name, email string) command.Command {
  return command.New(
    RegisterUserCmd,
    registerUserPayload{Name: name, Email: email},
    command.Aggregate(UserAggregate, id), // Bind the command to an aggregate
  )
}

func RegisterCommands(r *codec.Registry) {
  gr := codec.Gob(r)
  gr.GobRegister(RegisterUserCmd, func() interface{} {
    return registerUserPayload{}
  })
}
```

## Dispatch Commands

The `command.Bus.Dispatch` method dispatches commands between services. Commands
can be created with `command.New()`, which accepts the name of the command and
the command payload.

```go
package example

import (
  "github.com/modernice/goes/tree/main/command"
)

func dispatchCommand(bus command.Bus) {
  cmd := command.New("cmd-name", somePayload{...})
  err := bus.Dispatch(context.TODO(), cmd)
}
```

### Synchronous Dispatch

By default, dispatches are asynchronous, which means that dispatching a command
blocks only until a command handler has been selected and not until the command
was actually handled. This means that errors that happen during the command
handling are not reported back to the dispatcher. In order to block the dispatch
until the command has been handled, pass the `dispatch.Sync()` dispatch option.
When using this option, the `command.Bus.Dispatch` method also reports the error
of the command handler.

```go
package example

import (
  "github.com/modernice/goes/tree/main/command"
  "github.com/modernice/goes/tree/main/command/cmdbus/dispatch"
)

func syncDispatch(bus command.Bus) {
  cmd := command.New("cmd-name", somePayload{...})
  err := bus.Dispatch(context.TODO(), cmd, dispatch.Sync())
}
```

## Command Handling

The `command.Bus.Subscribe` method subscribes to commands and returns two
channels: one for the incoming commands and one for any asynchronous error of
the command bus. These allow for flexible handling of commands and errors, but
requires quite a bit of boilerplate code to actually handle commands. The
`command.Handler` provides convenience methods around the command bus to allow
for simpler command handling.

```go
// Package auth is an example authentication service.
package auth

// All the previous code ...

func HandleCommands(
  ctx context.Context,
  bus command.Bus,
  repo aggregate.Repository,
) <-chan error {
  h := command.NewHandler(bus)

  registerErrors := h.MustHandle(
    ctx, RegisterUserCmd,
    func(ctx command.Context) error {
      load := ctx.Payload().(registerUserPayload)

      u := NewUser(ctx.AggregateID())

      if err := repo.Fetch(ctx, u); err != nil {
        return fmt.Errorf("fetch user: %w [id=%v]", err, ctx.AggregateID())
      }

      if err := u.Register(load.Name, load.Email); err != nil {
        return err
      }

      if err := repo.Save(ctx, u); err != nil {
        return fmt.Errorf("save user: %w", err)
      }

      return nil
    },
  )

  return registerErrrors
}
```

## Built-in Commands

goes provides built-in, commonly needed commands and command handlers.

```go
package example

import (
  "github.com/modernice/goes/tree/main/aggregate/repository"
  "github.com/modernice/goes/tree/main/codec"
  "github.com/modernice/goes/tree/main/command/builtin"
  "github.com/modernice/goes/tree/main/command/cmdbus"
)

func example(ebus event.Bus) {
  ereg := codec.New() // Event registry
  creg := codec.New() // Command registry
  cbus := cmdbus.New(creg, ereg, ebus)
  repo := repository.New(ebus)
  
  builtin.RegisterEvents(ereg)
  builtin.RegisterCommands(creg)

  errs := builtin.MustHandle(context.TODO(), cbus, repo)

  for err := range errs {
    log.Printf("Failed to handle built-in command: %v")
  }
}
```

### Delete an Aggregate

The `builtin.DeleteAggregate()` command deletes an aggregate by deleting its
event stream from the event store.

```go
package example

import (
  "github.com/modernice/goes/tree/main/aggregate/repository"
  "github.com/modernice/goes/tree/main/codec"
  "github.com/modernice/goes/tree/main/command/builtin"
  "github.com/modernice/goes/tree/main/command/cmdbus"
)

func example(bus command.Bus) {
  userID := uuid.New() // Get this from somewhere
  cmd := builtin.DeleteAggregate("auth.user", userID)

  err := bus.Dispatch(context.TODO(), cmd)
  // handle err
}
```
