package xaggregate

import (
	"github.com/modernice/goes"
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
func Make[ID goes.ID](newID func() ID, n int, opts ...MakeOption) (
	_ []aggregate.AggregateOf[ID],
	getAppliedEvents func(ID) []event.Of[any, ID],
) {
	var cfg makeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	as := make([]aggregate.AggregateOf[ID], n)
	gaes := make(map[ID]func() []event.Of[any, ID])
	for i := range as {
		id := newID()
		a, gae := makeAggregate(cfg.name, id)
		as[i] = a
		gaes[id] = gae
	}
	return as, func(id ID) []event.Of[any, ID] {
		return gaes[id]()
	}
}

func makeAggregate[ID goes.ID](name string, id ID) (_ aggregate.AggregateOf[ID], getAppliedEvents func() []event.Of[any, ID]) {
	if name == "" {
		name = "foo"
	}
	var applied []event.Of[any, ID]
	a := test.NewAggregate(name, id, test.ApplyEventFunc("", func(e event.Of[any, ID]) {
		applied = append(applied, e)
	}))
	return a, func() []event.Of[any, ID] {
		events := make([]event.Of[any, ID], len(applied))
		copy(events, applied)
		return events
	}
}
