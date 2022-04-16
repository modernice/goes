package command

import (
	"github.com/google/uuid"
)

// Command is a command with arbitrary payload.
type Command = Of[any]

// A Command represents a command in the business model of an application or
// service. Commands can be dispatched through a Bus to handlers of such
// Commands.

// Of is a command with the given specific payload type. A command has a unique
// id, a name, and a user-provided payload. A command can optionally provide the
// aggregate that it acts on.
type Of[Payload any] interface {
	// ID returns the command id.
	ID() uuid.UUID

	// Name returns the command name.
	Name() string

	// Payload returns the command payload.
	Payload() Payload

	// Aggregate returns aggregate that the command acts on.
	Aggregate() (id uuid.UUID, name string)
}

// Option is an option for creating a command.
type Option func(*Cmd[any])

// Cmd is the command implementation.
type Cmd[Payload any] struct {
	Data Data[Payload]
}

// Data contains the fields of a Cmd.
type Data[Payload any] struct {
	ID            uuid.UUID
	Name          string
	Payload       Payload
	AggregateName string
	AggregateID   uuid.UUID
}

// ID returns an Option that overrides the auto-generated UUID of a command.
func ID(id uuid.UUID) Option {
	return func(b *Cmd[any]) {
		b.Data.ID = id
	}
}

// Aggregate returns an Option that links a command to an aggregate.
func Aggregate(name string, id uuid.UUID) Option {
	return func(b *Cmd[any]) {
		b.Data.AggregateName = name
		b.Data.AggregateID = id
	}
}

// New returns a new command with the given name and payload. A random UUID is
// generated and set as the command id.
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

// Aggregate returns the aggregate that the command acts on.
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

// TryCast tries to cast the payload of the given command to the given `To`
// type. If the payload is not of type `To`, false is returned.
func TryCast[To, From any](cmd Of[From]) (Cmd[To], bool) {
	load, ok := any(cmd.Payload()).(To)
	if !ok {
		return Cmd[To]{}, false
	}
	id, name := cmd.Aggregate()
	return New(cmd.Name(), load, ID(cmd.ID()), Aggregate(name, id)), true
}

// Cast casts the payload of the given command to the given `To` type. If the
// payload is not of type `To`, Cast panics.
func Cast[To, From any](cmd Of[From]) Cmd[To] {
	id, name := cmd.Aggregate()
	return New(cmd.Name(), any(cmd.Payload()).(To), ID(cmd.ID()), Aggregate(name, id))
}
