# goes - Event-Sourcing Framework

[![Go Reference](https://pkg.go.dev/badge/github.com/modernice/goes.svg)](https://pkg.go.dev/github.com/modernice/goes)
[![Test](https://github.com/modernice/goes/actions/workflows/test.yml/badge.svg)](https://github.com/modernice/goes/actions/workflows/test.yml)

`goes` is a collection of interfaces, tools, and backend implementations that
allow you to write event-sourced applicatios in Go.

If you have any questions or feedback, feel free to [open an issue](
https://github.com/modernice/goes/issues/new) or [start a discussion](
https://github.com/modernice/goes/discussions).

## Getting Started

### Installation

goes is not yet versioned because the API still changes too often. Install from
the main branch or from specific commit hash, and make sure to install nested
modules with `/...`

```sh
go get github.com/modernice/goes/...@main
```

```sh
go get github.com/modernice/goes/...@<commit-hash>
```

### Examples

- [To-Do App](./examples/todo)

## Introduction

This documentation assumes knowledge of [CQRS](
https://martinfowler.com/bliki/CQRS.html), [event-sourcing](
https://martinfowler.com/eaaDev/EventSourcing.html), and other
[related concepts](https://github.com/heynickc/awesome-ddd). Please make
yourself familiar with these before reading further.

### Features

- Event Store Implementations ([In-Memory](./event/eventstore),
	[MongoDB](./backend/mongo))
- Distributed Event Bus ([NATS Core](https://nats.io) /
	[NATS JetStream](https://docs.nats.io/nats-concepts/jetstream))
- Distributed, event-driven Command Bus
- [Aggregate Framework](./aggregate)
- [Projection Framework](./projection)
- [SAGAs](./saga)
- Pre-built Modules
	- [Authorization Module](./contrib/auth)

### Components

goes provides incrementally adoptable components that together form a complete
framework for building event-sourced applications. Read a component's README for
a guide on how to use it.

- [Event System](./event)
- [Command System](./command)
- [Aggregate Framework](./aggregate)
- [Projection Framework](./projection)
- [SAGAs](./saga)

### Backends

#### Event Bus

- [Channels (In-Memory)](./event/eventbus/chabus.go)
- [NATS Core](./backend/nats)
- [NATS JetStream](./backend/nats)

#### Event Store

- [In-Memory](./event/eventstore/memstore.go)
- [MongoDB](./backend/mongo)

## Contributing

_TBD_

## License

[Apache License, Version 2.0](./LICENSE)
