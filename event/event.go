// Package event defines the core event primitives used throughout goes. It
// provides helpers to create, transform, sort and test domain events and
// declares the interfaces that event stores and buses work with.
package event

import (
	"sort"
	"time"

	"github.com/google/uuid"
	qtime "github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/internal/xtime"
)

// All is a wildcard event name that matches every event.
const All = "*"

// #region event
// Event is an event whose data type is not known at compile time. It is a
// convenient alias for Of[any].
type Event = Of[any]

// Of represents a domain event whose payload has type Data. Every event has a unique ID, a name and a timestamp. If Aggregate returns non-zero values the event belongs to an aggregate stream; otherwise it is treated as an application event.
type Of[Data any] interface {
	// ID returns the unique identifier of the event.
	ID() uuid.UUID
	// Name reports the event's name.
	Name() string
	// Time reports when the event occurred.
	Time() time.Time
	// Data returns the event payload.
	Data() Data

	// Aggregate returns the aggregate ID, name and version. Zero values
	// indicate that the event does not belong to an aggregate.
	Aggregate() (id uuid.UUID, name string, version int)
}

// #endregion event

// Option configures an event created with New.
type Option func(*Evt[any])

// Evt implements Of for a specific payload type. It carries ID, name, time and
// optional aggregate metadata.
type Evt[D any] struct {
	D Data[D]
}

// Data is the serializable representation of an event with all metadata and the
// payload.
type Data[D any] struct {
	ID               uuid.UUID
	Name             string
	Time             time.Time
	Data             D
	AggregateName    string
	AggregateID      uuid.UUID
	AggregateVersion int
}

// ID sets the identifier of the event.
func ID(id uuid.UUID) Option {
	return func(evt *Evt[any]) {
		evt.D.ID = id
	}
}

// Time sets the timestamp of the event.
func Time(t time.Time) Option {
	return func(evt *Evt[any]) {
		evt.D.Time = t
	}
}

// Aggregate associates the event with the aggregate identified by id, name and
// version.
func Aggregate(id uuid.UUID, name string, version int) Option {
	return func(evt *Evt[any]) {
		evt.D.AggregateName = name
		evt.D.AggregateID = id
		evt.D.AggregateVersion = version
	}
}

// Previous copies aggregate information from prev and increments its version by
// one.
func Previous[Data any](prev Of[Data]) Option {
	id, name, v := prev.Aggregate()
	if id != uuid.Nil {
		v++
	}
	return Aggregate(id, name, v)
}

// New builds a new event with the given name and data and applies the provided
// options. A random ID and the current time are used if not explicitly set.
func New[D any](name string, data D, opts ...Option) Evt[D] {
	evt := Evt[any]{D: Data[any]{
		ID:   uuid.New(),
		Name: name,
		Time: xtime.Now(),
		Data: data,
	}}
	for _, opt := range opts {
		opt(&evt)
	}

	return Evt[D]{
		D: Data[D]{
			ID:               evt.D.ID,
			Name:             evt.D.Name,
			Time:             evt.D.Time,
			Data:             evt.D.Data.(D),
			AggregateName:    evt.D.AggregateName,
			AggregateID:      evt.D.AggregateID,
			AggregateVersion: evt.D.AggregateVersion,
		},
	}
}

// Equal reports whether all events share the same ID, name, time, data and
// aggregate information. Providing fewer than two events returns true.
func Equal(events ...Of[any]) bool {
	if len(events) < 2 {
		return true
	}
	first := events[0]
	fid, fname, fv := first.Aggregate()
	for _, evt := range events[1:] {
		if (evt == nil && first != nil) || (evt != nil && first == nil) {
			return false
		}

		id, name, v := evt.Aggregate()

		if !(evt.ID() == first.ID() &&
			evt.Name() == first.Name() &&
			evt.Time().Equal(first.Time()) &&
			evt.Data() == first.Data() &&
			id == fid &&
			name == fname &&
			v == fv) {
			return false
		}
	}
	return true
}

// Sort returns a sorted copy of events using the given Sorting and direction.
func Sort[Events ~[]Of[D], D any](events Events, sort Sorting, dir SortDirection) Events {
	return SortMulti(events, SortOptions{Sort: sort, Dir: dir})
}

// SortMulti sorts events by applying each sort option in order until one yields
// a difference; when all options compare equal the original order is preserved.
func SortMulti[Events ~[]Of[D], D any](events Events, sorts ...SortOptions) Events {
	sorted := make(Events, len(events))
	copy(sorted, events)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			cmp := CompareSorting(opts.Sort, sorted[i], sorted[j])
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}

// ID returns the event's identifier.
func (evt Evt[D]) ID() uuid.UUID {
	return evt.D.ID
}

// Name returns the event's name.
func (evt Evt[D]) Name() string {
	return evt.D.Name
}

// Time reports when the event occurred.
func (evt Evt[D]) Time() time.Time {
	return evt.D.Time
}

// Data returns the event payload.
func (evt Evt[D]) Data() D {
	return evt.D.Data
}

// Aggregate returns the aggregate metadata of the event. Zero values indicate a
// non-aggregate event.
func (evt Evt[D]) Aggregate() (uuid.UUID, string, int) {
	return evt.D.AggregateID, evt.D.AggregateName, evt.D.AggregateVersion
}

// Any converts the event to have an `any` payload.
func (evt Evt[D]) Any() Evt[any] {
	return Any[D](evt)
}

// Event returns the event itself as an Of[D].
func (evt Evt[D]) Event() Of[D] {
	return evt
}

// Any returns a copy of evt with its data type erased.
func Any[Data any](evt Of[Data]) Evt[any] {
	return Cast[any](evt)
}

// Cast converts the payload of evt to type To and returns a new event with the
// same metadata. It panics if the payload cannot be asserted to To.
func Cast[To, From any](evt Of[From]) Evt[To] {
	return New(
		evt.Name(),
		any(evt.Data()).(To),
		ID(evt.ID()),
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	)
}

// TryCast attempts to cast the payload of evt to type To and reports whether it
// succeeded.
func TryCast[To, From any](evt Of[From]) (Evt[To], bool) {
	data, ok := any(evt.Data()).(To)
	if !ok {
		return Evt[To]{}, false
	}

	return New(
		evt.Name(),
		data,
		ID(evt.ID()),
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	), true
}

// Expand converts an arbitrary Of[D] into an Evt[D]. If evt already is an
// Evt[D], it is returned unchanged.
func Expand[D any](evt Of[D]) Evt[D] {
	if evt, ok := evt.(Evt[D]); ok {
		return evt
	}
	return New(evt.Name(), evt.Data(), ID(evt.ID()), Time(evt.Time()), Aggregate(evt.Aggregate()))
}

// Test reports whether evt satisfies the constraints of q.
func Test[Data any](q Query, evt Of[Data]) bool {
	if q == nil {
		return true
	}

	if names := q.Names(); len(names) > 0 &&
		!stringsContains(names, evt.Name()) {
		return false
	}

	if ids := q.IDs(); len(ids) > 0 && !uuidsContains(ids, evt.ID()) {
		return false
	}

	if times := q.Times(); times != nil {
		if exact := times.Exact(); len(exact) > 0 &&
			!timesContains(exact, evt.Time()) {
			return false
		}
		if ranges := times.Ranges(); len(ranges) > 0 &&
			!testTimeRanges(ranges, evt.Time()) {
			return false
		}
		if min := times.Min(); !min.IsZero() && !testMinTimes(min, evt.Time()) {
			return false
		}
		if max := times.Max(); !max.IsZero() && !testMaxTimes(max, evt.Time()) {
			return false
		}
	}

	id, name, v := evt.Aggregate()

	if names := q.AggregateNames(); len(names) > 0 &&
		!stringsContains(names, name) {
		return false
	}

	if ids := q.AggregateIDs(); len(ids) > 0 &&
		!uuidsContains(ids, id) {
		return false
	}

	if versions := q.AggregateVersions(); versions != nil {
		if exact := versions.Exact(); len(exact) > 0 &&
			!intsContains(exact, v) {
			return false
		}
		if ranges := versions.Ranges(); len(ranges) > 0 &&
			!testVersionRanges(ranges, v) {
			return false
		}
		if min := versions.Min(); len(min) > 0 &&
			!testMinVersions(min, v) {
			return false
		}
		if max := versions.Max(); len(max) > 0 &&
			!testMaxVersions(max, v) {
			return false
		}
	}

	if aggregates := q.Aggregates(); len(aggregates) > 0 {
		var found bool
		for _, aggregate := range aggregates {
			if aggregate.Name == name &&
				(aggregate.ID == uuid.Nil || aggregate.ID == id) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func stringsContains(vals []string, val string) bool {
	for _, v := range vals {
		if v == val {
			return true
		}
	}
	return false
}

func uuidsContains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

func timesContains(times []time.Time, t time.Time) bool {
	for _, v := range times {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func intsContains(ints []int, i int) bool {
	for _, v := range ints {
		if v == i {
			return true
		}
	}
	return false
}

func testTimeRanges(ranges []qtime.Range, t time.Time) bool {
	for _, r := range ranges {
		if r.Includes(t) {
			return true
		}
	}
	return false
}

func testMinTimes(min time.Time, t time.Time) bool {
	if t.Equal(min) || t.After(min) {
		return true
	}
	return false
}

func testMaxTimes(max time.Time, t time.Time) bool {
	if t.Equal(max) || t.Before(max) {
		return true
	}
	return false
}

func testVersionRanges(ranges []version.Range, v int) bool {
	for _, r := range ranges {
		if r.Includes(v) {
			return true
		}
	}
	return false
}

func testMinVersions(min []int, v int) bool {
	for _, m := range min {
		if v >= m {
			return true
		}
	}
	return false
}

func testMaxVersions(max []int, v int) bool {
	for _, m := range max {
		if v <= m {
			return true
		}
	}
	return false
}
