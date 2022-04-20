package todo

import (
	"github.com/modernice/goes/codec"
)

// Events
const (
	TaskAdded   = "todo.list.task_added"
	TaskRemoved = "todo.list.task_removed"
	TasksDone   = "todo.list.tasks_done"
)

// ListEvents are all events of a todo list.
var ListEvents = [...]string{
	TaskAdded,
	TaskRemoved,
	TasksDone,
}

// TaskRemovedEvent is the event data for the TaskRemoved event.
//
// You can use any type as event data. In this case, the event data is a struct.
// The event data types for the TaskAdded and TasksDone events are `string` and
// `[]string` respectively.
type TaskRemovedEvent struct{ Task string }

// RegisterEvents registers events into a registry.
func RegisterEvents(r *codec.GobRegistry) {
	codec.GobRegister[string](r, TaskAdded)
	codec.GobRegister[TaskRemovedEvent](r, TaskRemoved)
	codec.GobRegister[[]string](r, TasksDone)
}
