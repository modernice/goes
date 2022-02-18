package cmdbus

import (
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/codec"
)

const (
	// CommandDispatched is published by a Bus to dispatch a Command.
	CommandDispatched = "goes.command.dispatched"

	// CommandRequested is published by a Bus to show interest in a dispatched
	// Command.
	CommandRequested = "goes.command.requested"

	// CommandAssigned is published by a Bus to assign a dispatched Command to a
	// Handler.
	CommandAssigned = "goes.command.assigned"

	// CommandAccepted is published by a Bus to notify other Buses that a
	// Command has been accepted.
	CommandAccepted = "goes.command.accepted"

	// CommandExecuted is published by a Bus to notify other Buses that a
	// Command has been executed.
	CommandExecuted = "goes.command.executed"
)

// CommandDispatchedData is the Event Data for the CommandDispatched Event.
type CommandDispatchedData[ID goes.ID] struct {
	// ID is the unique Command ID.
	ID ID

	// Name is the name of the Command.
	Name string

	// AggregateName is the name of the  Aggregate the Command belongs to.
	// (optional)
	AggregateName string

	// AggregateID is the ID of the Aggregate the Command belongs to. (optional)
	AggregateID ID

	// Payload is the encoded domain-specific Command Payload.
	Payload []byte
}

// CommandRequestedData is the Event Data for the CommandRequested Event.
type CommandRequestedData[ID goes.ID] struct {
	ID        ID
	HandlerID uuid.UUID
}

// CommandAssignedData is the Event Data for the CommandAssigned Event.
type CommandAssignedData[ID goes.ID] struct {
	ID        ID
	HandlerID uuid.UUID
}

// CommandAcceptedData is the Event Data for the CommandAccepted Event.
type CommandAcceptedData[ID goes.ID] struct {
	ID        ID
	HandlerID uuid.UUID
}

// CommandExecutedData is the Event Data for the CommandExecuted Event.
type CommandExecutedData[ID goes.ID] struct {
	ID      ID
	Runtime time.Duration
	Error   string
}

// RegisterEvents registers the command events into a Registry.
func RegisterEvents[ID goes.ID](reg *codec.Registry) {
	gob := codec.Gob(reg)
	gob.GobRegister(CommandDispatched, func() any {
		return CommandDispatchedData[ID]{}
	})
	gob.GobRegister(CommandRequested, func() any {
		return CommandRequestedData[ID]{}
	})
	gob.GobRegister(CommandAssigned, func() any {
		return CommandAssignedData[ID]{}
	})
	gob.GobRegister(CommandAccepted, func() any {
		return CommandAcceptedData[ID]{}
	})
	gob.GobRegister(CommandExecuted, func() any {
		return CommandExecutedData[ID]{}
	})
}
