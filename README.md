# goes - Event-Sourcing Toolkit

> This documentation is a work in progress. If you need help understanding the
components of this library, feel free [open an issue](
http://github.com/modernice/goes/issues) or [start a discussion](
http://github.com/modernice/goes/discussions). Feedback is always welcome.

`goes` is a collection of interfaces, tools and backend implementations that
allow you to write event-sourced applications in Go.

## Introduction

This documentation assumes knowledge of

- Event-Sourcing,
- Domain-Driven Design,
- and CQRS.

Please make [yourself familiar with these concepts](
https://github.com/heynickc/awesome-ddd) before reading further.

### Features

- Distributed Event Bus (using [NATS Core](http://nats.io) / [NATS JetStream](
  https://docs.nats.io/nats-concepts/jetstream))
- Distributed, event-driven Command Bus
- Event Store ([In-Memory](http://github.com/modernice/goes/tree/main/event/eventstore) or
  [MongoDB](http://github.com/modernice/goes/tree/main/backend/mongo))
- Projections
- SAGAs

### To-Do

- Testing Tools
  - Aggregates
  - Commands
  - Projections
- Development Tools
  - Code Generators (?)
  - Event Store CLI/UI
  - Projection CLI/UI
- Documentation
  - Examples / Guides
- Generics
  - [Helpers](./tree/main/helper)
  - `codec.Registry`
  - `event.Event` (?)
  - `command.Command` (?)

## Getting Started

### Installation

```sh
go get github.com/modernice/goes
```

### Examples

[~~A full example of an app can be found here.~~ (To-Do)](
http://github.com/modernice/goes/tree/main/examples)

### Guides

- [~~Setup Events~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/setupevents)
- [~~Publish & Subscribe to Events~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/pubsubevent)
- [~~Create & Test an Aggregate~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/aggregate)
- [~~Setup Commands~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/setupcommands)
- [~~Dispatch & Subscribe to Commands~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/pubsubcommand)
- [~~Create Projections~~ (To-Do)](http://github.com/modernice/goes/tree/main/examples/projections)

## Components

goes consists of multiple components that, when used together, provide a CQRS
and Event-Sourcing framework/toolkit. Read a component's README for usage guides.

### Event System

[github.com/modernice/goes/tree/main/event](http://github.com/modernice/goes/tree/main/event)

goes defines and implements a unified event system for both application events
and aggregate events.

### Aggregates

[github.com/modernice/goes/tree/main/aggregate](http://github.com/modernice/goes/tree/main/aggregate)

goes provides utilities to create event-sourced aggregates that build on top of
the event system.

### Command System

[github.com/modernice/goes/tree/main/command](http://github.com/modernice/goes/tree/main/command)

goes implements a distributed command bus that communicates between processes
over the event system.

### Projections

[github.com/modernice/goes/tree/main/projection](http://github.com/modernice/goes/tree/main/projection)

The `projection` package provides utilities for creating and managing
projections over events.

### SAGAs

[github.com/modernice/goes/tree/main/saga](http://github.com/modernice/goes/tree/main/saga)

The `saga` package implements a SAGA coordinator / process manager for more
complex multi-step transactions.

## Backends

### Event Bus

- [Channels (In-Memory)](http://github.com/modernice/goes/tree/main/event/eventbus/chabus.go)
- [NATS Core](http://github.com/modernice/goes/tree/main/backend/nats)
- [NATS JetStream](http://github.com/modernice/goes/tree/main/backend/nats)

### Event Store

- [In-Memory](http://github.com/modernice/goes/tree/main/event/eventstore/memstore.go)
- [MongoDB](http://github.com/modernice/goes/tree/main/backend/mongo)

## Contributing

_TBD (Contributions welcome)_

## License

[MIT](./LICENSE)
