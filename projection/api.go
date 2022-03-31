package projection

import (
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// EventApplier should be implemented by projections. It is the main projection
// interface and is used to apply events onto the projection.
type EventApplier[Data any] interface {
	ApplyEvent(event.Of[Data])
}

// A ProgressAware projection keeps track of its projection progress in terms of
// the time of the last applied event.
//
// *Progressor implements ProgressAware, and can be embedded in your projections.
//
// The current progress of a projection is the Time of the last applied event.
// A projection that provides its projection progress only receives events with
// a Time that is after the current progress Time.
type ProgressAware interface {
	// Progress returns the projection's progress as the Time of the last
	// applied event.
	Progress() time.Time

	// SetProgress sets the progress of the projection to the provided Time.
	SetProgress(time.Time)
}

// Progressor can be embedded into a projection to implement the ProgressAware API.
type Progressor struct {
	LatestEventTime int64
}

// NewProgressor returns a new *Progressor that can be embeded into a projection
// to implement the ProgressAware API.
func NewProgressor() *Progressor {
	return &Progressor{}
}

// Progress returns the projection progress in terms of the time of the latest
// applied event. If p.LatestEventTime is 0, the zero Time is returned.
func (p *Progressor) Progress() time.Time {
	if p.LatestEventTime == 0 {
		return time.Time{}
	}
	return time.Unix(0, p.LatestEventTime)
}

// SetProgress sets the projection progress as the time of the latest applied event.
func (p *Progressor) SetProgress(t time.Time) {
	if t.IsZero() {
		p.LatestEventTime = 0
		return
	}
	p.LatestEventTime = t.UnixNano()
}

// A Resetter is a projection that can reset its state. Projections that
// implement Resetter can be reset by projection jobs before applying events
// onto the projection. Projection jobs reset a projection if the WithReset()
// option was used to create the job.
type Resetter interface {
	// Reset should implement any custom logic to reset the state of a projection.
	Reset()
}

// Guard can be implemented by projections to "guard" the projection from
// illegal events. If a projection implements Guard, GuardProjection(evt)
// is called for every event that should be applied onto the projection to
// determine if the event is allows to be applied. If GuardProjection(evt)
// returns false, the event is not applied.
//
// QueryGuard implements Guard.
type Guard interface {
	// GuardProjection determines whether an Event is allowed to be applied onto a projection.
	GuardProjection(event.Event) bool
}

// QueryGuard is a Guard that uses an event query to determine if an event is
// allowed to be applied onto a projection.
//
//	type MyProjection struct {
//		projection.QueryGuard
//	}
//
//	func NewMyProjection() *MyProjection {
//		return &MyProjection{
//			QueryGuard: query.New(query.Name("foo", "bar", "baz")), // allow "foo", "bar", and "baz" events
//		}
//	}
type QueryGuard query.Query

// GuardProjection tests the Guard's Query against a given Event and returns
// whether the Event is allowed to be applied onto the projection.
func (g QueryGuard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}

// GuardFunc allows functions to be used as Guards.
type GuardFunc func(event.Event) bool

// GuardProjection returns guard(evt).
func (guard GuardFunc) GuardProjection(evt event.Event) bool {
	return guard(evt)
}

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
//	func (idx SearchIndex) ApplyEvent(event.Of[D]) { ... }
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
//		return streams.Walk(ctx, func(r aggregate.Ref) error {
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
