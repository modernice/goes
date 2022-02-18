package command

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
)

var (
	// ErrAlreadyFinished is returned when a Command is finished multiple times.
	ErrAlreadyFinished = errors.New("command already finished")
)

type Command = Of[any, uuid.UUID]

// A Command represents a command in the business model of an application or
// service. Commands can be dispatched through a Bus to handlers of such
// Commands.
type Of[Payload any, ID goes.ID] interface {
	// ID returns the Command ID.
	ID() ID

	// Name returns the Command name.
	Name() string

	// Payload returns the Command Payload.
	Payload() Payload

	// Aggregate returns the attached aggregate data.
	Aggregate() (id ID, name string)
}

// A Bus dispatches Commands to appropriate handlers.
type Bus[ID goes.ID] interface {
	// Dispatch sends the Command to the appropriate subscriber. Dispatch must
	// only return nil if the Command has been successfully received by a
	// subscriber.
	Dispatch(context.Context, Of[any, ID], ...DispatchOption) error

	// Subscribe subscribes to Commands with the given names and returns a
	// channel of Contexts. Implementations of Bus must ensure that Commands
	// aren't received by multiple subscribers.
	Subscribe(ctx context.Context, names ...string) (<-chan ContextOf[any, ID], <-chan error, error)
}

// Config is the configuration for dispatching a Command.
type DispatchConfig struct {
	// A synchronous dispatch waits for the execution of the Command to finish
	// and returns the execution error if there was any.
	//
	// A dispatch is automatically made synchronous when Repoter is non-nil.
	Synchronous bool

	// If Reporter is not nil, the Bus will report the execution result of a
	// Command to Reporter by calling Reporter.Report(). The reporter must
	// implement Reporter.
	//
	// A non-nil Reporter makes the dispatch synchronous.
	Reporter interface{}
}

// DispatchOption is an option for dispatching Commands.
type DispatchOption func(*DispatchConfig)

// A Reporter reports execution results of a Command.
type Reporter[ID goes.ID] interface {
	Report(report.Report[ID])
}

type Context = ContextOf[any, uuid.UUID]

// Context is the context for handling Commands.
type ContextOf[P any, ID goes.ID] interface {
	context.Context
	Of[P, ID]

	// AggregateID returns the UUID of the attached aggregate, or uuid.Nil.
	AggregateID() ID

	// AggregateName returns the name of the attached aggregate, or an empty string.
	AggregateName() string

	// Finish should be called after the Command has been handled so that the
	// Bus that dispatched the Command can be notified about the execution
	// result.
	Finish(context.Context, ...finish.Option) error
}

// Option is a command option.
type Option func(*Cmd[any, goes.AID])

// Cmd is the implementation of Command.
type Cmd[Payload any, ID goes.ID] struct {
	Data Data[Payload, ID]
}

// Data contains the actual fields of a Cmd.
type Data[Payload any, ID goes.ID] struct {
	ID            ID
	Name          string
	Payload       Payload
	AggregateName string
	AggregateID   ID
}

// Aggregate returns an Option that links a Command to an Aggregate.
func Aggregate[Payload any, ID goes.ID](name string, id ID) Option {
	return func(b *Cmd[any, goes.AID]) {
		b.Data.AggregateName = name
		b.Data.AggregateID = goes.AnyID(id)
	}
}

// New returns a new command with the given name and payload.
func New[P any, ID goes.ID](id ID, name string, pl P, opts ...Option) Cmd[P, ID] {
	cmd := Cmd[any, goes.AID]{
		Data: Data[any, goes.AID]{
			ID:      goes.AnyID(id),
			Name:    name,
			Payload: pl,
		},
	}
	for _, opt := range opts {
		opt(&cmd)
	}

	var aggregateID ID
	if cmd.Data.AggregateID.ID != nil {
		aggregateID = cmd.Data.AggregateID.ID.(ID)
	}

	return Cmd[P, ID]{
		Data: Data[P, ID]{
			ID:            id,
			Name:          cmd.Data.Name,
			Payload:       cmd.Data.Payload.(P),
			AggregateName: cmd.Data.AggregateName,
			AggregateID:   aggregateID,
		},
	}
}

// ID returns the command id.
func (cmd Cmd[P, ID]) ID() ID {
	return cmd.Data.ID
}

// Name returns the command name.
func (cmd Cmd[P, ID]) Name() string {
	return cmd.Data.Name
}

// Payload returns the command payload.
func (cmd Cmd[P, ID]) Payload() P {
	return cmd.Data.Payload
}

// Aggregate returns the attached aggregate data.
func (cmd Cmd[P, ID]) Aggregate() (ID, string) {
	return cmd.Data.AggregateID, cmd.Data.AggregateName
}

// Any returns the command with its type paramter set to `any`.
func (cmd Cmd[P, ID]) Any() Cmd[any, ID] {
	return Any[P, ID](cmd)
}

// Command returns the command as an interface.
func (cmd Cmd[P, ID]) Command() Of[P, ID] {
	return cmd
}

// Any returns the command with its type paramter set to `any`.
func Any[P any, ID goes.ID](cmd Of[P, ID]) Cmd[any, ID] {
	id, name := cmd.Aggregate()
	return New[any](cmd.ID(), cmd.Name(), cmd.Payload(), Aggregate[P](name, id))
}

func TryCast[To, From any, ID goes.ID](cmd Of[From, ID]) (Cmd[To, ID], bool) {
	load, ok := any(cmd.Payload()).(To)
	if !ok {
		return Cmd[To, ID]{}, false
	}
	id, name := cmd.Aggregate()
	return New(cmd.ID(), cmd.Name(), load, Aggregate[To](name, id)), true
}

func Cast[To, From any, ID goes.ID](cmd Of[From, ID]) Cmd[To, ID] {
	id, name := cmd.Aggregate()
	return New(cmd.ID(), cmd.Name(), any(cmd.Payload()).(To), Aggregate[To](name, id))
}
