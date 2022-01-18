package xaggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
)

// MakeOption is a Make option.
type MakeOption func(*makeConfig)

type makeConfig struct {
	name string
	opts []aggregate.Option
}

// Name returns a MakeOption that specifies the AggregateName.
func Name(name string) MakeOption {
	return func(cfg *makeConfig) {
		cfg.name = name
	}
}

// Make returns n "foo" aggregates and a function to retrieve the applied events
// for those aggregates.
func Make(n int, opts ...MakeOption) (
	_ []aggregate.Aggregate,
	getAppliedEvents func(uuid.UUID) []event.EventOf[any],
) {
	var cfg makeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	as := make([]aggregate.Aggregate, n)
	gaes := make(map[uuid.UUID]func() []event.EventOf[any])
	for i := range as {
		id := uuid.New()
		a, gae := makeAggregate(cfg.name, id)
		as[i] = a
		gaes[id] = gae
	}
	return as, func(id uuid.UUID) []event.EventOf[any] {
		return gaes[id]()
	}
}

func makeAggregate(name string, id uuid.UUID) (_ aggregate.Aggregate, getAppliedEvents func() []event.EventOf[any]) {
	if name == "" {
		name = "foo"
	}
	var applied []event.EventOf[any]
	a := test.NewAggregate(name, id, test.ApplyEventFunc("", func(e event.EventOf[any]) {
		applied = append(applied, e)
	}))
	return a, func() []event.EventOf[any] {
		events := make([]event.EventOf[any], len(applied))
		copy(events, applied)
		return events
	}
}
