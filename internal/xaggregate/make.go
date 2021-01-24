package xaggregate

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
)

// Make returns n "foo" aggregates and a function to retrieve the applied events
// for those aggregates.
func Make(n int) (
	_ []aggregate.Aggregate,
	getAppliedEvents func(uuid.UUID) []event.Event,
) {
	as := make([]aggregate.Aggregate, n)
	gaes := make(map[uuid.UUID]func() []event.Event)
	for i := range as {
		a, gae := makeAggregate(uuid.New())
		as[i] = a
		gaes[a.AggregateID()] = gae
	}
	return as, func(id uuid.UUID) []event.Event {
		return gaes[id]()
	}
}

func makeAggregate(id uuid.UUID) (_ aggregate.Aggregate, getAppliedEvents func() []event.Event) {
	var applied []event.Event
	foo := test.NewFoo(id, test.ApplyEventFunc("", func(e event.Event) {
		applied = append(applied, e)
	}))
	return foo, func() []event.Event {
		events := make([]event.Event, len(applied))
		copy(events, applied)
		return events
	}
}
