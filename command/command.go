package command

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/report"
	"github.com/modernice/goes/command/finish"
)

var (
	// ErrAlreadyFinished is returned when a Command is finished multiple times.
	ErrAlreadyFinished = errors.New("command already finished")
)

type Command = Of[any]

// A Command represents a command in the business model of an application or
// service. Commands can be dispatched through a Bus to handlers of such
// Commands.
type Of[Payload any] interface {
	// ID returns the Command ID.
	ID() uuid.UUID

	// Name returns the Command name.
	Name() string

	// Payload returns the Command Payload.
	Payload() Payload

	// Aggregate returns the attached aggregate data.
	Aggregate() (id uuid.UUID, name string)
}

// A Bus dispatches Commands to appropriate handlers.
type Bus interface {
	// Dispatch sends the Command to the appropriate subscriber. Dispatch must
	// only return nil if the Command has been successfully received by a
	// subscriber.
	Dispatch(context.Context, Command, ...DispatchOption) error

	// Subscribe subscribes to Commands with the given names and returns a
	// channel of Contexts. Implementations of Bus must ensure that Commands
	// aren't received by multiple subscribers.
	Subscribe(ctx context.Context, names ...string) (<-chan Ctx[any], <-chan error, error)
}

// Config is the configuration for dispatching a Command.
type DispatchConfig struct {
	// A synchronous dispatch waits for the execution of the Command to finish
	// and returns the execution error if there was any.
	//
	// A dispatch is automatically made synchronous when Repoter is non-nil.
	Synchronous bool

	// If Reporter is not nil, the Bus will report the execution result of a
	// Command to Reporter by calling Reporter.Report().
	//
	// A non-nil Reporter makes the dispatch synchronous.
	Reporter Reporter
}

// DispatchOption is an option for dispatching Commands.
type DispatchOption func(*DispatchConfig)

// A Reporter reports execution results of a Command.
type Reporter interface {
	Report(report.Report)
}

type Context = Ctx[any]

// Context is the context for handling Commands.
type Ctx[P any] interface {
	context.Context
	Of[P]

	// AggregateID returns the UUID of the attached aggregate, or uuid.Nil.
	AggregateID() uuid.UUID

	// AggregateName returns the name of the attached aggregate, or an empty string.
	AggregateName() string

	// Finish should be called after the Command has been handled so that the
	// Bus that dispatched the Command can be notified about the execution
	// result.
	Finish(context.Context, ...finish.Option) error
}

// Option is a command option.
type Option func(*Cmd[any])

// Cmd is the implementation of Command.
type Cmd[Payload any] struct {
	Data Data[Payload]
}

// Data contains the actual fields of a Cmd.
type Data[Payload any] struct {
	ID            uuid.UUID
	Name          string
	Payload       Payload
	AggregateName string
	AggregateID   uuid.UUID
}

// ID returns an Option that overrides the auto-generated UUID of a Command.
func ID(id uuid.UUID) Option {
	return func(b *Cmd[any]) {
		b.Data.ID = id
	}
}

// Aggregate returns an Option that links a Command to an aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *Cmd[any]) {
		b.Data.AggregateName = name
		b.Data.AggregateID = id
	}
}

// New returns a new command with the given name and payload.
func New[P any](name string, pl P, opts ...Option) Cmd[P] {
	cmd := Cmd[any]{
		Data: Data[any]{
			ID:      uuid.New(),
			Name:    name,
			Payload: pl,
		},
	}
	for _, opt := range opts {
		opt(&cmd)
	}
	return Cmd[P]{
		Data: Data[P]{
			ID:            cmd.Data.ID,
			Name:          cmd.Data.Name,
			Payload:       cmd.Data.Payload.(P),
			AggregateName: cmd.Data.AggregateName,
			AggregateID:   cmd.Data.AggregateID,
		},
	}
}

// ID returns the command id.
func (cmd Cmd[P]) ID() uuid.UUID {
	return cmd.Data.ID
}

// Name returns the command name.
func (cmd Cmd[P]) Name() string {
	return cmd.Data.Name
}

// Payload returns the command payload.
func (cmd Cmd[P]) Payload() P {
	return cmd.Data.Payload
}

// Aggregate returns the attached aggregate data.
func (cmd Cmd[P]) Aggregate() (uuid.UUID, string) {
	return cmd.Data.AggregateID, cmd.Data.AggregateName
}

// Any returns the command with its type paramter set to `any`.
func (cmd Cmd[P]) Any() Cmd[any] {
	return Any[P](cmd)
}

// Command returns the command as an interface.
func (cmd Cmd[P]) Command() Of[P] {
	return cmd
}

// Any returns the command with its type paramter set to `any`.
func Any[P any](cmd Of[P]) Cmd[any] {
	id, name := cmd.Aggregate()
	return New[any](cmd.Name(), cmd.Payload(), ID(cmd.ID()), Aggregate(name, id))
}

func TryCast[To, From any](cmd Of[From]) (Cmd[To], bool) {
	load, ok := any(cmd.Payload()).(To)
	if !ok {
		return Cmd[To]{}, false
	}
	id, name := cmd.Aggregate()
	return New(cmd.Name(), load, ID(cmd.ID()), Aggregate(name, id)), true
}

func Cast[To, From any](cmd Of[From]) Cmd[To] {
	id, name := cmd.Aggregate()
	return New(cmd.Name(), any(cmd.Payload()).(To), ID(cmd.ID()), Aggregate(name, id))
}
