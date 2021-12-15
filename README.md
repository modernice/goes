# goes – CQRS & event-sourcing toolkit

**This library is still in development and probably not production ready.**

`goes` is a toolkit for creating distributed, event-sourced applications in Go.

Using the building blocks and tools provided by this library, you can quickly
build event-sourced apps without having to write a single line of infrastructure
code. goes provides:

- Distributed Event Bus (using [NATS](https://nats.io) /
  [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream) /
  [NATS Streaming](https://docs.nats.io/legacy/stan))
- Distributed, event-driven Command Bus
- Event store (using [MongoDB](https://www.mongodb.com))
- Projections (continuously & periodically)
- SAGAs (process managers)

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
