package todo

import (
	"github.com/modernice/goes/codec"
)

const (
	TaskAdded   = "todo.list.task_added"
	TaskRemoved = "todo.list.task_removed"
	TasksDone   = "todo.list.tasks_done"
)

var TaskEvents = [...]string{
	TaskAdded,
	TaskRemoved,
	TasksDone,
}

// TaskRemovedEvent is the event data for TaskRemoved.
//
// You can use any type as event data. In this example the event data is a
// struct. If you look below you can see that the TaskAdded and TasksDone events
// use other types for their event data.
type TaskRemovedEvent struct{ Task string }

// RegisterEvents registers events into a registry.
func RegisterEvents(r *codec.GobRegistry) {
	codec.GobRegister[string](r, TaskAdded)
	codec.GobRegister[TaskRemovedEvent](r, TaskRemoved)
	codec.GobRegister[[]string](r, TasksDone)
}
