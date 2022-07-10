# Example – To-Do App

This example shows how to implement a "todo list" app. The app consists of a
"todo" server and a client that sends commands to the server.

- [`list.go`](./list.go) implements the "todo list" aggregate
- [`events.go`](./events.go) defines and registers the "todo list" events
- [`commands.go`](./commands.go) defines and registers the "todo list" commands
- [`counter.go`](./counter.go) implements a read model projection
- [`cmd/server`](./cmd/server) is the server app
- [`cmd/client`](./cmd/client) is the client app

## Details

### Backends

- NATS Core (Event Bus)
- MongoDB (Event Store)

## Build & Run

Requires Docker.

### Default setup

```sh
make build && make default
```

### Debounced projection

This setup sets the `TODO_DEBOUNCE` environment variable to `1s`, resulting in
a single "batch"-update of the `Counter` projection.

```sh
make build && make debounce
```
