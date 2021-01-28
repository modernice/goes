package factory

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

var (
	// ErrUnknownName is returned when trying to make an Aggregate with a name
	// that's unknown to the Factory.
	ErrUnknownName = errors.New("unknown aggregate name")
)

// Option is a Factory option.
type Option func(*factory)

type factory struct {
	funcs map[string]func(uuid.UUID) aggregate.Aggregate
}

// For returns an Option that specifies the factory function for Aggregates with
// the given name.
func For(name string, fn func(uuid.UUID) aggregate.Aggregate) Option {
	return func(f *factory) {
		f.funcs[name] = fn
	}
}

// New returns a new Factory.
func New(opts ...Option) aggregate.Factory {
	f := factory{funcs: make(map[string]func(uuid.UUID) aggregate.Aggregate)}
	for _, opt := range opts {
		opt(&f)
	}
	return &f
}

func (f *factory) Make(name string, id uuid.UUID) (aggregate.Aggregate, error) {
	fn, ok := f.funcs[name]
	if !ok {
		if fn, ok = f.funcs[""]; !ok {
			return nil, fmt.Errorf("make %s(%s): %w", name, id, ErrUnknownName)
		}
	}
	return fn(id), nil
}
