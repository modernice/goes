package command

//go:generate mockgen -destination=./mocks/command.go . Command,Encoder,Bus,Handler

import (
	"context"
	"io"

	"github.com/google/uuid"
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
	Dispatch(context.Context, Command) error
}

// A Handler handles Commands.
type Handler interface {
	// Handle handles a Command.
	Handle(Context, Command) error
}

// HandlerFunc allows functions to be used as Handlers.
type HandlerFunc func(Context, Command) error

// Context is the context for Handlers.
type Context interface {
	context.Context
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
