# Example: To-Do List

This example uses a "todo list" to show how to define and test an event-sourced
aggregate and commands for the aggregate.

- `events.go` defines the events of the list
- `commands.go` defines and handles commands that can be dispatched by other
	services
- `list.go` implements the "todo list" aggregate
- `repos.go` defines the repositories
- `cmd/server` is the todo server
- `cmd/client` is an example client that dispatches commands to the server

## Used Backends

- NATS Core (Event Bus)
- MongoDB (Event Store)

## Build & Run

Requires Docker.

```sh
make build && make up
```
