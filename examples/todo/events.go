package todo

import (
	"github.com/modernice/goes/codec"
)

const (
	TaskAdded   = "todo.list.task_added"
	TaskRemoved = "todo.list.task_removed"
	TaskDone    = "todo.list.task_done"
)

type TaskAddedEvent struct{ Task string }

type TaskRemovedEvent struct{ Task string }

type TaskDoneEvent struct{ Task string }

func RegisterEvents(r *codec.GobRegistry) {
	r.GobRegister(TaskAdded, func() any { return TaskAddedEvent{} })
	r.GobRegister(TaskRemoved, func() any { return TaskRemovedEvent{} })
	r.GobRegister(TaskDone, func() any { return TaskDoneEvent{} })
}
