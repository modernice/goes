package command

//go:generate mockgen -destination=./mocks/command.go . Command,Bus

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

// A Command represents a command in the business model of an application or
// service. Commands can be dispatched through a Bus to handlers of such
// Commands.
type Command interface {
	// ID returns the Command ID.
	ID() uuid.UUID

	// Name returns the Command name.
	Name() string

	// Payload returns the Command Payload.
	Payload() interface{}

	// Aggregate returns the attached aggregate data.
	Aggregate() (uuid.UUID, string)
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
	Subscribe(ctx context.Context, names ...string) (<-chan Context, <-chan error, error)
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

// Context is the context for handling Commands.
type Context interface {
	context.Context
	Command

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
type Option func(*Cmd)

// Cmd is the implementation of Command.
type Cmd struct {
	Data Data
}

// Data contains the actual fields of Cmd.
type Data struct {
	ID            uuid.UUID
	Name          string
	Payload       interface{}
	AggregateName string
	AggregateID   uuid.UUID
}

// ID returns an Option that overrides the auto-generated UUID of a Command.
func ID(id uuid.UUID) Option {
	return func(b *Cmd) {
		b.Data.ID = id
	}
}

// Aggregate returns an Option that links a Command to an Aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *Cmd) {
		b.Data.AggregateName = name
		b.Data.AggregateID = id
	}
}

// New returns a new command with the given name and payload.
func New(name string, pl interface{}, opts ...Option) Cmd {
	cmd := Cmd{
		Data: Data{
			ID:      uuid.New(),
			Name:    name,
			Payload: pl,
		},
	}
	for _, opt := range opts {
		opt(&cmd)
	}
	return cmd
}

// ID returns the command id.
func (cmd Cmd) ID() uuid.UUID {
	return cmd.Data.ID
}

// Name returns the command name.
func (cmd Cmd) Name() string {
	return cmd.Data.Name
}

// Payload returns the command payload.
func (cmd Cmd) Payload() interface{} {
	return cmd.Data.Payload
}

// Aggregate returns the attached aggregate data.
func (cmd Cmd) Aggregate() (uuid.UUID, string) {
	return cmd.Data.AggregateID, cmd.Data.AggregateName
}
