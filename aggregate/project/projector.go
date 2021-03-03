package project

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
)

// A Projector builds Projections.
type Projector interface {
	// Project builds the given Projection p.
	//
	// Project first fetches all Events for the Aggregate with the name
	// p.AggregateName() and UUID p.AggregateID(), then applies those Events on
	// the Projection by calling p.ApplyEvent(e) for every Event e.
	Project(context.Context, Projection) error
}

type projector struct {
	events event.Store
}

// NewProjector returns a Projector. It uses the provided event.Store to query
// for the Events of a given Aggregate.
//
// Example:
//
//	type ProjectedFoo struct {
//		project.Projection
//	}
//
//	func (pf *ProjectedFoo) ApplyEvent(evt event.Event) {
//		// apply event ...
//	}
//
//	var proj project.Projector
//	p := &ProjectedFoo{Projection: project.New("foo", uuid.New())}
//	err := proj.Project(context.TODO(), p)
//	// handle err
func NewProjector(events event.Store) Projector {
	return &projector{events}
}

func (proj *projector) Project(ctx context.Context, p Projection) error {
	str, err := proj.events.Query(ctx, equery.New(
		equery.AggregateName(p.AggregateName()),
		equery.AggregateID(p.AggregateID()),
		equery.SortByMulti(
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}
	defer str.Close(ctx)

	for str.Next(ctx) {
		p.ApplyEvent(str.Event())
	}

	if err = str.Err(); err != nil {
		return fmt.Errorf("event stream: %w", err)
	}

	return nil
}
