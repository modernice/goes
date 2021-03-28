package command

//go:generate mockgen -destination=./mocks/command.go . Command,Encoder,Bus,Registry

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/finish"
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
	Payload() Payload

	// AggregateName returns the Aggregates name the Command belongs to.
	// (optional)
	AggregateName() string

	// AggregateID returns the Aggregates UUID the Command belongs to.
	// (optional)
	AggregateID() uuid.UUID
}

// Payload is the payload of a Command.
type Payload interface{}

// An Encoder encodes and decodes Payloads.
type Encoder interface {
	// Encode encodes the given Payload for a Command with the given name and
	// writes the result into the Writer.
	Encode(io.Writer, string, Payload) error

	// Decode decodes the Payload for a Command with the given name from the
	// provided Reader.
	Decode(string, io.Reader) (Payload, error)
}

// A Registry is an Encoder that also allows to register new Commands into it
// and instantiate Payloads from Command names.
type Registry interface {
	Encoder

	// Register registers the Payload factory for Commands with the given name.
	Register(string, func() Payload)

	// New makes and returns a fresh Payload for a Command with the given name.
	New(string) (Payload, error)
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
	Report(Report)
}

// A Report provides information about the execution of a Command.
type Report interface {
	// Command returns the executed Command.
	Command() Command

	// Runtime returns the runtime of the execution.
	Runtime() time.Duration

	// Err returns the error that made the execution fail.
	Err() error
}

// Context is the context for handling Commands.
type Context interface {
	context.Context

	// Command returns the actual Command.
	Command() Command

	// Finish should be called after the Command has been handled so that the
	// Bus that dispatched the Command can be notified about the execution
	// result.
	Finish(context.Context, ...finish.Option) error
}

// Option is a Command option.
type Option func(*command)

type command struct {
	id            uuid.UUID
	name          string
	payload       Payload
	aggregateName string
	aggregateID   uuid.UUID
}

// ID returns an Option that overrides the auto-generated UUID of a Command.
func ID(id uuid.UUID) Option {
	return func(b *command) {
		b.id = id
	}
}

// Aggregate returns an Option that links a Command to an Aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *command) {
		b.aggregateName = name
		b.aggregateID = id
	}
}

// New returns a new Command with the given name and Payload.
func New(name string, pl Payload, opts ...Option) Command {
	cmd := command{
		id:      uuid.New(),
		name:    name,
		payload: pl,
	}
	for _, opt := range opts {
		opt(&cmd)
	}
	return cmd
}

func (cmd command) ID() uuid.UUID {
	return cmd.id
}

func (cmd command) Name() string {
	return cmd.name
}

func (cmd command) Payload() Payload {
	return cmd.payload
}

func (cmd command) AggregateName() string {
	return cmd.aggregateName
}

func (cmd command) AggregateID() uuid.UUID {
	return cmd.aggregateID
}
