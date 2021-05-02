# goes – CQRS & Event-sourcing toolkit

**This library is still in development and probably not production ready
(although it is already used in production).**

`goes` is a toolkit for creating distributed, event-sourced applications in Go
that enables you to focus on implementing your business logic and not having to
constantly (re)write infrastructure code along the way. Some key components
provided by goes are:

- Distributed Event Bus ([NATS](https://nats.io) / NATS Streaming)
- Distributed, Event-driven Command Bus (NATS / NATS Streaming)
- Event store implementation (MongoDB)
- Projections
- SAGAs

## Getting started

### Install

```sh
go get github.com/modernice/goes
```

### Usage

Documentation hasn't been written yet, but check out the
[ecommerce example](./examples/ecommerce) which makes use of most of the tools
provided by goes. The example has many comments that go into detail on how you
could use goes.

## License

[MIT](./LICENSE)
