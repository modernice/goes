# Commands

Package `command` defines and implements a command system for distributed
applications. Communication between processes is accomplished using the
underlying [event system](../event).

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

### Subscribe to commands

*TBD*

### Dispatch a command

*TBD*

## Command handling

### Standalone command handler

*TBD*

### Aggregate-based command handler

*TBD*

## Things to consider

### Load-balancing

*TBD*

### Long-running commands

*TBD*
