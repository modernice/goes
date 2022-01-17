package aggregate

//go:generate mockgen -source=aggregate.go -destination=./mocks/aggregate.go

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/xtime"
)

// Deprecated: Use Ref instead.
type Tuple = Ref

// Ref is a reference to a specific aggregate, identified by its name and id.
type Ref = event.AggregateRef

// Option is an Aggregate option.
type Option[D any] func(*Base[D])

// Base can be embedded into structs to make them implement Aggregate.
type Base[D any] struct {
	ID      uuid.UUID
	Name    string
	Version int
	Changes []event.Event[D]
}

// Version returns an Option that sets the version of an Aggregate.
func Version[D any](v int) Option[D] {
	return func(b *Base[D]) {
		b.Version = v
	}
}

// ApplyHistory applies the given events to the aggregate a to reconstruct the
// state of a at the time of the latest event. If the aggregate implements
// Committer, a.TrackChange(events) and a.Commit() are called before returning.
func ApplyHistory[D any](a Aggregate[D], events ...event.Event[D]) error {
	if err := ValidateConsistency(a, events...); err != nil {
		return fmt.Errorf("validate consistency: %w", err)
	}
	for _, evt := range events {
		a.ApplyEvent(evt)
	}

	if c, ok := a.(Committer[D]); ok {
		c.TrackChange(events...)
		c.Commit()
	}

	return nil
}

// Sort sorts aggregates and returns the sorted aggregates.
func Sort[D any](as []Aggregate[D], s Sorting, dir SortDirection) []Aggregate[D] {
	return SortMulti(as, SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts aggregates by multiple fields and returns the sorted
// aggregates.
func SortMulti[D any](as []Aggregate[D], sorts ...SortOptions) []Aggregate[D] {
	sorted := make([]Aggregate[D], len(as))
	copy(sorted, as)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			cmp := opts.Sort.Compare(ToAny(sorted[i]), ToAny(sorted[j]))
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}

// UncommittedVersion returns the version the aggregate, including any uncommitted changes.
func UncommittedVersion[D any](a Aggregate[D]) int {
	_, _, v := a.Aggregate()
	return v + len(a.AggregateChanges())
}

// NextVersion returns the next (uncommitted) version of an aggregate (UncommittedVersion(a) + 1).
func NextVersion[D any](a Aggregate[D]) int {
	return UncommittedVersion(a) + 1
}

// NextEvent makes and returns the next Event e for the aggregate a. NextEvent
// calls a.ApplyEvent(e) and a.TrackChange(e) before returning the Event.
//
//	var foo aggregate.Aggregate
//	evt := aggregate.NextEvent(foo, "event-name", ...)
func NextEvent[D any](a Aggregate[D], name string, data D, opts ...event.Option[D]) event.Event[D] {
	aid, aname, _ := a.Aggregate()

	opts = append([]event.Option[D]{
		event.Aggregate[D](
			aid,
			aname,
			NextVersion(a),
		),
		event.Time[D](nextTime(a)),
	}, opts...)

	evt := event.New(name, data, opts...)

	a.ApplyEvent(evt)

	if c, ok := a.(Committer[D]); ok {
		c.TrackChange(evt)
	}

	return evt
}

// HasChange returns whether Aggregate a has an uncommitted Event with the given name.
func HasChange[D any](a Aggregate[D], eventName string) bool {
	for _, change := range a.AggregateChanges() {
		if change.Name() == eventName {
			return true
		}
	}
	return false
}

func ToAny[D any](a Aggregate[D]) (out Aggregate[any]) {
	if a, ok := any(a).(Aggregate[any]); ok {
		return a
	}
	panic(fmt.Errorf("%T is not a %T", a, out))
}

// New returns a new Base for an Aggregate.
func New[D any](name string, id uuid.UUID, opts ...Option[D]) *Base[D] {
	b := &Base[D]{
		ID:   id,
		Name: name,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Base[D]) Aggregate() (uuid.UUID, string, int) {
	return b.ID, b.Name, b.Version
}

// AggregateID implements Aggregate.
func (b *Base[D]) AggregateID() uuid.UUID {
	return b.ID
}

// AggregateName implements Aggregate.
func (b *Base[D]) AggregateName() string {
	return b.Name
}

// AggregateVersion implements Aggregate.
func (b *Base[D]) AggregateVersion() int {
	return b.Version
}

// AggregateChanges implements Aggregate.
func (b *Base[D]) AggregateChanges() []event.Event[D] {
	return b.Changes
}

// TrackChange implements Aggregate.
func (b *Base[D]) TrackChange(events ...event.Event[D]) {
	b.Changes = append(b.Changes, events...)
}

// Commit implement Aggregate.
func (b *Base[D]) Commit() {
	if len(b.Changes) == 0 {
		return
	}
	// b.TrackChange guarantees a correct event order, so we can safely assume
	// the last element has the highest version.
	b.Version = event.PickAggregateVersion(b.Changes[len(b.Changes)-1])
	b.Changes = b.Changes[:0]
}

// ApplyEvent implements aggregate. Aggregates that embed *Base should overide
// ApplyEvent.
func (*Base[D]) ApplyEvent(event.Event[D]) {}

// SetVersion implements snapshot.Aggregate.
func (b *Base[D]) SetVersion(v int) {
	b.Version = v
}

// nextTime returns the Time for the next event of the given aggregate. The time
// should most of the time just be time.Now(), but nextTime guarantees that the
// returned Time is at least 1 nanosecond after the previous event. This is
// necessary for the projection.Progressing API to work.
func nextTime[D any](a Aggregate[D]) time.Time {
	changes := a.AggregateChanges()
	now := xtime.Now()
	if len(changes) == 0 {
		return now
	}
	latestTime := changes[len(changes)-1].Time()
	nowTrunc := now.Truncate(0)
	if nowTrunc.Equal(latestTime) || nowTrunc.Before(latestTime.Truncate(0)) {
		return changes[len(changes)-1].Time().Add(time.Nanosecond)
	}
	return now
}
