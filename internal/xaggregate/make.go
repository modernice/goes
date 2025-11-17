package xaggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal"
)

// MakeOption is a Make option.
type MakeOption func(*makeConfig)

type makeConfig struct {
	name string
	opts []aggregate.Option
}

// Name returns a MakeOption that specifies the aggregateName.
func Name(name string) MakeOption {
	return func(cfg *makeConfig) {
		cfg.name = name
	}
}

// Make returns n "foo" aggregates and a function to retrieve the applied events
// for those aggregates.
func Make(n int, opts ...MakeOption) (
	_ []aggregate.Aggregate,
	getAppliedEvents func(uuid.UUID) []event.Event,
) {
	var cfg makeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	as := make([]aggregate.Aggregate, n)
	gaes := make(map[uuid.UUID]func() []event.Event)
	for i := range as {
		id := internal.NewUUID()
		a, gae := makeAggregate(cfg.name, id)
		as[i] = a
		gaes[id] = gae
	}
	return as, func(id uuid.UUID) []event.Event {
		return gaes[id]()
	}
}

func makeAggregate(name string, id uuid.UUID) (_ aggregate.Aggregate, getAppliedEvents func() []event.Event) {
	if name == "" {
		name = "foo"
	}
	var applied []event.Event
	a := test.NewAggregate(name, id, test.ApplyEventFunc("", func(e event.Event) {
		applied = append(applied, e)
	}))
	return a, func() []event.Event {
		events := make([]event.Event, len(applied))
		copy(events, applied)
		return events
	}
}
