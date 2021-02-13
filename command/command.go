package command

//go:generate mockgen -destination=./mocks/command.go . Command,Encoder,Bus,Context

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/done"
)

// A Command is a (domain) command that can be dispatched through a Bus.
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

// Payload is the payload for a Command.
type Payload interface{}

// An Encoder encodes and decodes Payloads.
type Encoder interface {
	// Encode encodes the given Payload and writes the result into the Writer.
	Encode(io.Writer, Payload) error

	// Decode decodes the Payload in Reader r.
	Decode(name string, r io.Reader) (Payload, error)
}

// A Bus dispatches Commands to the appropriate Handlers.
type Bus interface {
	// Dispatch sends the Command to the appropriate Handlers. Which Handlers
	// receive the Command can vary and depends on the underlying Bus
	// implementation.
	Dispatch(context.Context, Command, ...dispatch.Option) error

	// Handle returns a channel of Contexts. When a Command whose name is in
	// names is dispatched, that Command is received from the Context channel.
	// Implementations of Bus must ensure that a Command will only be received
	// by a single channel/handler.
	Handle(ctx context.Context, names ...string) (<-chan Context, error)
}

// Context is the context for handling Commands.
type Context interface {
	// Command returns the actual Command.
	Command() Command

	// Done should be called after the execution of the Command to report the
	// execution result. Use Options to add information about the execution to
	// the report.
	Done(context.Context, ...done.Option) error
}

// Option is a Command option.
type Option func(*base)

type base struct {
	id            uuid.UUID
	name          string
	payload       Payload
	aggregateName string
	aggregateID   uuid.UUID
}

// ID returns an Option that overrides the auto-generated UUID of a Command.
func ID(id uuid.UUID) Option {
	return func(b *base) {
		b.id = id
	}
}

// Aggregate returns an Option that links a Command to an Aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *base) {
		b.aggregateName = name
		b.aggregateID = id
	}
}

// New returns a new Command with the given name and Payload.
func New(name string, pl Payload, opts ...Option) Command {
	cmd := base{
		id:      uuid.New(),
		name:    name,
		payload: pl,
	}
	for _, opt := range opts {
		opt(&cmd)
	}
	return &cmd
}

func (cmd *base) ID() uuid.UUID {
	return cmd.id
}

func (cmd *base) Name() string {
	return cmd.name
}

func (cmd *base) Payload() Payload {
	return cmd.payload
}

func (cmd *base) AggregateName() string {
	return cmd.aggregateName
}

func (cmd *base) AggregateID() uuid.UUID {
	return cmd.aggregateID
}
