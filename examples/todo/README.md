# Example: To-Do List

This example uses a "todo list" to show how to define and test an event-sourced
aggregate and commands for the aggregate.

- `events.go` defines the events of the list
- `list.go` implements the list aggregate
- `commands.go` defines and handles commands that can be dispatched by other
	services
