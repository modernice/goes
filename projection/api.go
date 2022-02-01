package projection

import (
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// An EventApplier applies events onto itself to build the projection state.
type EventApplier interface {
	ApplyEvent(event.Event)
}

// // Progressing makes projections track their projection progress.
// //
// // Embed *Progressor into a projection type to implement this interface.
// //
// // The current progress of a projection is the Time of the last applied event.
// // A projection that provides its projection progress only receives events with
// // a Time that is after the current progress Time.
// type Progressing interface {
// 	// Progress returns the projection's progress as the Time of the last
// 	// applied event.
// 	Progress() time.Time

// 	// SetProgress sets the progress of the projection to the provided Time.
// 	SetProgress(time.Time)
// }

// Progressing makes projections track their projection progress.
//
// Embed *Progressor into a projection type to implement this interface.
type Progressing interface {
	// Progress returns the progress of the projection in terms of the time and
	// version of the latest applied event. If the returned event version is 0
	// but the time is non-zero, only the time is used to compare the projection
	// progress. Otherwise both the time and version are used to compare. The
	// event version is additionally used to fetch missing events from the event
	// store when a projection job is applied onto the projection type.
	Progress() (time.Time, int)

	// TrackProgress sets the progress of the projection to the provided time
	// and version.
	TrackProgress(time.Time, int)
}

// Progressor can be embedded into a projection to implement the Progressing interface.
type Progressor struct {
	LatestEventTime    int64
	LatestEventVersion int
}

// Progress returns the projection progress in terms of the time and version of
// the latest applied event.
func (p *Progressor) Progress() (time.Time, int) {
	var t time.Time
	if p.LatestEventTime != 0 {
		t = time.Unix(0, p.LatestEventTime)
	}
	return t, p.LatestEventVersion
}

// TrackProgress sets the projection progress as the time and version of the latest applied event.
func (p *Progressor) TrackProgress(t time.Time, v int) {
	p.LatestEventVersion = v
	if t.IsZero() {
		p.LatestEventTime = 0
	} else {
		p.LatestEventTime = t.UnixNano()
	}
}

// A Resetter is a projection that can reset its state.
type Resetter interface {
	// Reset should implement any custom logic to reset the state of a
	// projection besides resetting the progress (if the projection implements
	// Progressing).
	Reset()
}

// Guard can be implemented by projection to prevent the application of an
// events. When a projection p implements Guard, p.GuardProjection(e) is called
// for every Event e and prevents the p.ApplyEvent(e) call if GuardProjection
// returns false.
type Guard interface {
	// GuardProjection determines whether an Event is allowed to be applied onto a projection.
	GuardProjection(event.Event) bool
}

// type Filter interface{}

// A QueryGuard is an event query that determines which Events are allows to be
// applied onto a projection.

// QueryGuard is a Guard that used an event query to determine the events that
// are allowed to be applied onto a projection.
type QueryGuard query.Query

// GuardFunc allows functions to be used as Guards.
type GuardFunc func(event.Event) bool

// HistoryDependent can be implemented by continuous projections that need the
// full event history (of the events that are configured in the Schedule) instead
// of just the events that triggered the continuous projection.
//
// Example:
//
//	// A Product is a product of a specific shop.
//	type Product struct {
//		*aggregate.Base
//
//		ShopID uuid.UUID
//		Name string
//	}
//
//	// SearchIndex projections the product catalog of a specific shop.
//	type SearchIndex struct {
//		shopID      uuid.UUID
//		products    []Product
//		initialized bool
//	}
//
//	func (idx SearchIndex) ApplyEvent(event.Event) { ... }
//
//	// RequiresFullHistory implements projection.HistoryDependent. If the
// 	// projection hasn't been run yet, the full history of the events is
//	// required for the projection.
//	func (idx SearchIndex) RequiresFullHistory() bool {
//		return !idx.initialized
//	}
//
//	var repo agggregate.Repository
//	s := schedule.Continuously(bus, store, []string{"product.created"})
//	s.Subscribe(context.TODO(), func(ctx projection.Job) error {
//		str, errs, err := ctx.Aggregates(ctx)
//
//		done := make(map[uuid.UUID]bool) // SearchIndexes that have been projected
//
//		return aggregate.WalkRefs(ctx, func(r aggregate.Ref) error {
//			p := &Product{Base: aggregate.New("product", r.ID)}
//			err := repo.Fetch(ctx, p)
//			shopID := p.ShopID
//
//			if done[shopID] {
//				return nil
//			}
//			done[shopID] = true
//
//			// Fetch the current SearchIndex for the given shop.
//			// If it does not exist yet, create the SearchIndex.
//			var idx *SearchIndex
//
//			// if idx.initialized == false, ctx.Apply fetches all past events.
//			if err := ctx.Apply(ctx, idx); err != nil {
//				return err
//			}
//
//			// Set initialized to true (after the first run of the projection).
//			idx.initialized = true
//
//			return nil
//		}, str, errs)
//	})
type HistoryDependent interface {
	RequiresFullHistory() bool
}

// GuardProjection returns guard(evt).
func (guard GuardFunc) GuardProjection(evt event.Event) bool {
	return guard(evt)
}

// GuardProjection tests the Guard's Query against a given Event and returns
// whether the Event is allowed to be applied onto the projection.
func (g QueryGuard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}

// ProjectionFilter returns the Query in a 1-element slice.
func (g QueryGuard) ProjectionFilter() []event.Query {
	return []event.Query{query.Query(g)}
}
