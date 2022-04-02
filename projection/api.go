package projection

import (
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// EventApplier should be implemented by projections. It is the main projection
// interface and is used to apply events onto the projection.
type EventApplier[Data any] interface {
	ApplyEvent(event.Of[Data])
}

// A ProgressAware projection keeps track of its projection progress in terms of
// the time and ids of the last applied events. When applying events onto a
// projection with projection.Apply(), only those events with a later time than
// the current progress time are applied to the projection.
//
// *Progressor implements ProgressAware, and can be embedded in your projections.
type ProgressAware interface {
	// Progress returns the projection progress in terms of the time of the last
	// applied events and the id's of those events. In most cases, Progress()
	// should only a single event id. Multiple event ids should be returned if
	// more than one event was applied at the returned time.
	Progress() (time.Time, []uuid.UUID)

	// SetProgress sets the projection progress in terms of the time of the last
	// applied event. The ids of the last applied events should be provided to
	// SetProgress().
	SetProgress(time.Time, ...uuid.UUID)
}

// Progressor can be embedded into a projection to implement ProgressAware.
type Progressor struct {
	// Time of the last applied events as elapsed nanoseconds since January 1, 1970 UTC.
	LastEventTime int64

	// LastEvents are the ids of last applied events that have LastEventTime as their time.
	LastEvents []uuid.UUID
}

// NewProgressor returns a new *Progressor that can be embeded into a projection
// to implement the ProgressAware API.
func NewProgressor() *Progressor {
	return &Progressor{LastEvents: make([]uuid.UUID, 0)}
}

// Progress returns the projection progress in terms of the time and ids of the
// last applied events. If p.LastEventTime is 0, the zero Time is returned.
func (p *Progressor) Progress() (time.Time, []uuid.UUID) {
	var t time.Time
	if p.LastEventTime > 0 {
		t = time.Unix(0, p.LastEventTime)
	}
	return t, p.LastEvents
}

// SetProgress sets the projection progress as the time of the latest applied
// event. The ids of the applied events that have the given time should be
// provided.
func (p *Progressor) SetProgress(t time.Time, ids ...uuid.UUID) {
	p.LastEvents = ids
	if t.IsZero() {
		p.LastEventTime = 0
	} else {
		p.LastEventTime = t.UnixNano()
	}
}

// A Resetter is a projection that can reset its state. projections that
// implement Resetter can be reset by projection jobs before applying events
// onto the projection. projection jobs reset a projection if the WithReset()
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
	// GuardProjection determines whether an event is allowed to be applied onto a projection.
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

// GuardProjection tests the Guard's Query against a given event and returns
// whether the event is allowed to be applied onto the projection.
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
