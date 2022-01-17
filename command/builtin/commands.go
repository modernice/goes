package builtin

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
)

// DeleteAggregateCmd is the name of the DeleteAggregate command.
const DeleteAggregateCmd = "goes.command.aggregate.delete"

// DeleteAggregatePayload is the command payload for deleting an aggregate.
type DeleteAggregatePayload struct{}

// DeleteAggregate returns the command to delete an aggregate. When using the
// built-in command handler of this package, aggregates are deleted by deleting
// their events from the event store. Additionally, a "goes.command.aggregate.deleted"
// is published after deletion.
func DeleteAggregate(name string, id uuid.UUID) command.Cmd[DeleteAggregatePayload] {
	return command.New(DeleteAggregateCmd, DeleteAggregatePayload{}, command.Aggregate[DeleteAggregatePayload](id, name))
}

// RegisterCommands registers the built-in commands into a command registry.
func RegisterCommands(r *codec.Registry[any]) {
	gob := codec.Gob(r)
	gob.GobRegister(DeleteAggregateCmd, func() interface{} { return DeleteAggregatePayload{} })
}
