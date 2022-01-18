package todo

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/streams"
)

const (
	AddTaskCmd    = "todo.list.add_task"
	RemoveTaskCmd = "todo.list.remove_task"
	DoneTaskCmd   = "todo.list.done_task"
)

// AddTask returns the command to add the given task to the given todo list.
func AddTask(listID uuid.UUID, task string) command.Cmd[string] {
	return command.New(AddTaskCmd, task, command.Aggregate[string](listID, ListAggregate))
}

// RemoveTask removes the command to remove the given task from the given todo list.
func RemoveTask(listID uuid.UUID, task string) command.Cmd[string] {
	return command.New(RemoveTaskCmd, task, command.Aggregate[string](listID, ListAggregate))
}

type donePayload struct {
	Tasks []string
}

// DoneTasks returns the command to mark the given tasks within the given list a done.
func DoneTasks(listID uuid.UUID, tasks ...string) command.Cmd[donePayload] {
	return command.New(DoneTaskCmd, donePayload{tasks}, command.Aggregate[donePayload](listID, ListAggregate))
}

// RegisterCommands registers commands into a registry.
func RegisterCommands(r *codec.GobRegistry) {
	r.GobRegister(AddTaskCmd, func() any { return "" })
	r.GobRegister(RemoveTaskCmd, func() any { return "" })
	r.GobRegister(DoneTaskCmd, func() any { return donePayload{} })
}

// HandleCommands handles commands until ctx is canceled. Any asynchronous
// errors that happen during the command handling are reported to the returned
// error channel.
func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.User) <-chan error {
	addErrors := command.MustHandle(ctx, bus, AddTaskCmd, func(ctx command.ContextOf[string]) error {
		list := New(ctx.AggregateID())
		return repo.Use(ctx, list, func() error {
			return list.Add(ctx.Payload())
		})
	})

	removeErrors := command.MustHandle(ctx, bus, RemoveTaskCmd, func(ctx command.ContextOf[string]) error {
		list := New(ctx.AggregateID())
		return repo.Use(ctx, list, func() error {
			return list.Remove(ctx.Payload())
		})
	})

	doneErrors := command.MustHandle(ctx, bus, DoneTaskCmd, func(ctx command.ContextOf[donePayload]) error {
		list := New(ctx.AggregateID())
		return repo.Use(ctx, list, func() error {
			return list.Done(ctx.Payload().Tasks...)
		})
	})

	return streams.FanInContext(ctx, addErrors, removeErrors, doneErrors)
}
