package project

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
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
	q := p.EventQuery()
	if q == nil {
		panic("nil Query")
	}

	str, errs, err := proj.events.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			return fmt.Errorf("event stream: %w", err)
		case evt, ok := <-str:
			if !ok {
				return nil
			}
			p.ApplyEvent(evt)
		}
	}
}
