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
- Event Store ([In-Memory](http://github.com/modernice/goes/event/eventstore) or
  [MongoDB](http://github.com/modernice/goes/backend/mongo))
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

## Getting Started

### Installation

```sh
go get github.com/modernice/goes
```

### Examples

[~~A full example of an app can be found here.~~](
http://github.com/modernice/goes/examples) (not yet)

### Guides

- [Setup Events](http://github.com/modernice/goes/examples/setupevents)
- [Publish & Subscribe to Events](http://github.com/modernice/goes/examples/pubsubevent)
- [Create & Test an Aggregate](http://github.com/modernice/goes/examples/aggregate)
- [Setup Commands](http://github.com/modernice/goes/examples/setupcommands)
- [Dispatch & Subscribe to Commands](http://github.com/modernice/goes/examples/pubsubcommand)
- [Create Projections](http://github.com/modernice/goes/examples/projections)

## Components

goes consists of multiple components that, when used together, provide a CQRS
and Event-Sourcing framework/toolkit. Read a component's README for usage guides.

### Event System

[github.com/modernice/goes/event](http://github.com/modernice/goes/event)

goes defines and implements a unified event system for both application events
and aggregate events.

### Aggregates

[github.com/modernice/goes/aggregate](http://github.com/modernice/goes/aggregate)

goes provides utilities to create event-sourced aggregates that build on top of
the event system.

### Command System

[github.com/modernice/goes/command](http://github.com/modernice/goes/command)

goes implements a distributed command bus that communicates between processes
over the event system.

### Projections

[github.com/modernice/goes/projection](http://github.com/modernice/goes/projection)

The `projection` package provides utilities for creating and managing
projections over events.

### SAGAs

[github.com/modernice/goes/saga](http://github.com/modernice/goes/saga)

The `saga` package implements a SAGA coordinator / process manager for more
complex multi-step transactions.

## Backends

### Event Bus

- [Channels (In-Memory)](http://github.com/modernice/goes/event/eventbus/chabus.go)
- [NATS Core](http://github.com/modernice/goes/backend/nats)
- [NATS JetStream](http://github.com/modernice/goes/backend/nats)

### Event Store

- [In-Memory](http://github.com/modernice/goes/event/eventstore/memstore.go)
- [MongoDB](http://github.com/modernice/goes/backend/mongo)

## Contributing

_TBD (Contributions welcome)_

## License

[MIT](./LICENSE)
