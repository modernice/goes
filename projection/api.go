package projection

import (
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// Target consumes events to build read state. Embed [Base] to satisfy this
// interface in typical projections.
type Target[Data any] interface {
	ApplyEvent(event.Of[Data])
}

// ProgressAware records the timestamp and ids of the last applied events.
// [Apply] skips events that are not newer than the stored progress. Embed
// [Progressor] for a default implementation.
type ProgressAware interface {
	// Progress reports the time and ids of the most recently applied events.
	Progress() (time.Time, []uuid.UUID)
	// SetProgress stores the time and ids of the last applied events.
	SetProgress(time.Time, ...uuid.UUID)
}

// Progressor is a basic [ProgressAware] implementation that can be embedded.
type Progressor struct {
	// LastEventTime holds the timestamp of the last applied events in Unix
	// nanoseconds.
	LastEventTime int64
	// LastEvents are the ids applied at LastEventTime.
	LastEvents []uuid.UUID
}

// NewProgressor returns a Progressor with zero progress.
func NewProgressor() *Progressor {
	return &Progressor{LastEvents: make([]uuid.UUID, 0)}
}

// Progress returns the current progress. A zero time means no events were
// applied yet.
func (p *Progressor) Progress() (time.Time, []uuid.UUID) {
	var t time.Time
	if p.LastEventTime > 0 {
		t = time.Unix(0, p.LastEventTime)
	}
	return t, p.LastEvents
}

// SetProgress records the time and ids of the most recent events.
func (p *Progressor) SetProgress(t time.Time, ids ...uuid.UUID) {
	p.LastEvents = ids
	if t.IsZero() {
		p.LastEventTime = 0
	} else {
		p.LastEventTime = t.UnixNano()
	}
}

// Resetter can wipe its state when a job requests a reset.
type Resetter interface {
	// Reset clears the projection's state.
	Reset()
}

// Guard may reject events before they are applied.
type Guard interface {
	// GuardProjection reports whether an event may be applied.
	GuardProjection(event.Event) bool
}

// QueryGuard allows events that satisfy an [event/query] Query.
type QueryGuard query.Query

// GuardProjection tests the underlying query against evt.
func (g QueryGuard) GuardProjection(evt event.Event) bool {
	return query.Test(query.Query(g), evt)
}

// GuardFunc adapts a function to [Guard].
type GuardFunc func(event.Event) bool

// GuardProjection calls the wrapped function.
func (guard GuardFunc) GuardProjection(evt event.Event) bool {
	return guard(evt)
}
