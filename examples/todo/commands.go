package todo

import (
	"context"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/handler"
)

// Commands
const (
	AddTaskCmd    = "todo.list.add_task"
	RemoveTaskCmd = "todo.list.remove_task"
	DoneTaskCmd   = "todo.list.done_task"
)

// AddTask returns the command to add the given task to the given todo list.
func AddTask(listID uuid.UUID, task string) command.Cmd[string] {
	return command.New(AddTaskCmd, task, command.Aggregate(ListAggregate, listID))
}

// RemoveTask removes the command to remove the given task from the given todo list.
func RemoveTask(listID uuid.UUID, task string) command.Cmd[string] {
	return command.New(RemoveTaskCmd, task, command.Aggregate(ListAggregate, listID))
}

// DoneTasks returns the command to mark the given tasks within the given list a done.
func DoneTasks(listID uuid.UUID, tasks ...string) command.Cmd[[]string] {
	return command.New(DoneTaskCmd, tasks, command.Aggregate(ListAggregate, listID))
}

// RegisterCommands registers commands into a registry.
func RegisterCommands(r codec.Registerer) {
	codec.Register[string](r, AddTaskCmd)
	codec.Register[string](r, RemoveTaskCmd)
	codec.Register[[]string](r, DoneTaskCmd)
}

// HandleCommands handles todo list commands that are dispatched over the
// provided command bus until ctx is canceled. Command errors are sent into
// the returned error channel.
func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) <-chan error {
	return handler.New(New, repo, bus).MustHandle(ctx)
}
