package project

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// A Projection is a read-model of an Aggregate.
type Projection interface {
	// ApplyEvent applies an Event on the Projection.
	ApplyEvent(event.Event)

	// EventQuery must return a Query which returns the Events that should be
	// applied onto the Projection.
	EventQuery() event.Query
}

type eventProjection struct {
	names []string
}

type aggregateProjection struct {
	names []string
}

// Events returns an embeddable Projection that projects every Event with the
// given names.
func Events(names ...string) Projection {
	return &eventProjection{names}
}

// Aggregates returns an embeddable Projection that projects Events of
// Aggregates with the given names.
func Aggregates(names ...string) Projection {
	return &aggregateProjection{names}
}

func (p *eventProjection) ApplyEvent(event.Event) {}

func (p *eventProjection) EventQuery() event.Query {
	return query.New(
		query.Name(p.names...),
		query.SortByMulti(
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	)
}

func (p *aggregateProjection) ApplyEvent(event.Event) {}

func (p *aggregateProjection) EventQuery() event.Query {
	return query.New(
		query.AggregateName(p.names...),
		query.SortByMulti(
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	)
}
