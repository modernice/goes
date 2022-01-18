package todo

import (
	"github.com/modernice/goes/codec"
)

const (
	TaskAdded   = "todo.list.task_added"
	TaskRemoved = "todo.list.task_removed"
	TaskDone    = "todo.list.task_done"
)

type (
	TaskAddedEvent   struct{ Task string }
	TaskRemovedEvent struct{ Task string }
	TaskDoneEvent    struct{ Tasks []string }
)

// RegisterEvents registers events into a registry.
func RegisterEvents(r *codec.GobRegistry) {
	r.GobRegister(TaskAdded, func() any { return TaskAddedEvent{} })
	r.GobRegister(TaskRemoved, func() any { return TaskRemovedEvent{} })
	r.GobRegister(TaskDone, func() any { return TaskDoneEvent{} })
}
