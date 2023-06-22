package event

import (
	"sort"
	"time"

	"github.com/google/uuid"
	qtime "github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/internal/xtime"
)

// All is a special event name that matches all events.
const All = "*"

// #region event
// Event is an event with arbitrary data.
type Event = Of[any]

// Of is an event with the given specific data type. An event has a unique id,
// a name, user-provided event data, and the time at which the event was raised.
//
// If the Aggregate method of an event returns non-zero values, the event is
// considered to belong to the event stream of that aggregate:
//
//	var evt event.Event
//	id, name, version := evt.Aggregate()
//	// id is the UUID of the aggregate that the event belongs to
//	// name is the name of the aggregate that the event belongs to
//	// version is the optimistic concurrency version of the event within the
//	// event stream of the aggregate
//
// If an event is not part of an aggregate, the Aggregate method should return
// only zero values.
//
// Use the New function to create an event:
//
//	evt := event.New("foo", 3)
//	// evt.Name() == "foo"
//	// evt.Data() == 3
//	// evt.Time() == time.Now()
//
// To create an event for an aggregate, use the Aggregate() option:
//
//	var aggregateID uuid.UUID
//	var aggregateName string
//	var aggregateVersion int
//	evt := event.New("foo", 3, event.Aggregate(aggregateID, aggregateName, aggregateVersion))
type Of[Data any] interface {
	// ID returns the id of the event.
	ID() uuid.UUID
	// Name returns the name of the event.
	Name() string
	// Time returns the time of the event.
	Time() time.Time
	// Data returns the event data.
	Data() Data

	// Aggregate returns the id, name and version of the aggregate that the
	// event belongs to. aggregate should return zero values if the event is not
	// an aggregate event.
	Aggregate() (id uuid.UUID, name string, version int)
}

// #endregion event

type Option func(*Evt[any])

// Evt is a concrete implementation of the Of interface, representing an event
// with specific data type and associated metadata such as the event's ID, name,
// time, and aggregate information. Evt can be used to create, manipulate, and
// test events in a type-safe manner.
type Evt[D any] struct {
	D Data[D]
}

// Data is a struct that holds event information such as its unique ID, name,
// time, and arbitrary data. Additionally, it contains aggregate-related fields
// like AggregateName, AggregateID, and AggregateVersion.
type Data[D any] struct {
	ID               uuid.UUID
	Name             string
	Time             time.Time
	Data             D
	AggregateName    string
	AggregateID      uuid.UUID
	AggregateVersion int
}

// ID returns the unique identifier of the event.
func ID(id uuid.UUID) Option {
	return func(evt *Evt[any]) {
		evt.D.ID = id
	}
}

// Time sets the time of an event to the provided time value. It is an Option
// function used when creating a new event with the New function.
func Time(t time.Time) Option {
	return func(evt *Evt[any]) {
		evt.D.Time = t
	}
}

// Aggregate returns the id, name, and version of the aggregate that the event
// belongs to. If the event is not an aggregate event, it should return zero
// values.
func Aggregate(id uuid.UUID, name string, version int) Option {
	return func(evt *Evt[any]) {
		evt.D.AggregateName = name
		evt.D.AggregateID = id
		evt.D.AggregateVersion = version
	}
}

// Previous sets the aggregate information for an event based on the provided
// previous event, incrementing the aggregate version by 1. It returns an Option
// to be used when creating a new event with New.
func Previous[Data any](prev Of[Data]) Option {
	id, name, v := prev.Aggregate()
	if id != uuid.Nil {
		v++
	}
	return Aggregate(id, name, v)
}

// New creates a new event with the specified name and data, applying any
// provided options such as ID, Time, or Aggregate information. It returns an
// Evt struct containing the event data and metadata.
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

// Equal returns true if all provided events have the same ID, name, time, data,
// and aggregate information. It returns false otherwise. If less than two
// events are provided, it returns true.
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

// Sort sorts the provided events according to the specified Sorting and
// SortDirection, returning a new slice of sorted events.
func Sort[Events ~[]Of[D], D any](events Events, sort Sorting, dir SortDirection) Events {
	return SortMulti(events, SortOptions{Sort: sort, Dir: dir})
}

// SortMulti sorts a slice of events based on the provided sort options, in the
// order they appear. If multiple events have the same value for a specified
// sort option, they will be sorted based on the next sort option in the list.
// If all sort options are equal, the original order is preserved.
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

// ID returns the unique identifier of the event.
func (evt Evt[D]) ID() uuid.UUID {
	return evt.D.ID
}

// Name returns the name of the event.
func (evt Evt[D]) Name() string {
	return evt.D.Name
}

// Time returns the time at which the event was raised.
func (evt Evt[D]) Time() time.Time {
	return evt.D.Time
}

// Data returns the event data of the Evt.
func (evt Evt[D]) Data() D {
	return evt.D.Data
}

// Aggregate returns the id, name, and version of the aggregate that the event
// belongs to. If the event is not an aggregate event, it returns zero values.
func (evt Evt[D]) Aggregate() (uuid.UUID, string, int) {
	return evt.D.AggregateID, evt.D.AggregateName, evt.D.AggregateVersion
}

// Any converts an event with a specific data type (Of[Data]) to an event with
// the generic any data type (Evt[any]).
func (evt Evt[D]) Any() Evt[any] {
	return Any[D](evt)
}

// Event returns the unique id, name, user-provided event data, and the time at
// which the event was raised. If the Aggregate method of an event returns
// non-zero values, the event is considered to belong to the event stream of
// that aggregate.
func (evt Evt[D]) Event() Of[D] {
	return evt
}

// Any returns an Evt[any] that is a copy of the given Of[Data] event with its
// data type erased. This can be useful when working with heterogeneous event
// lists where the specific data type is not important.
func Any[Data any](evt Of[Data]) Evt[any] {
	return Cast[any](evt)
}

// Cast converts an event with data of type From to an event with data of type
// To. The new event has the same ID, name, time, and aggregate information as
// the original event, but the data is cast to the specified To type.
func Cast[To, From any](evt Of[From]) Evt[To] {
	return New(
		evt.Name(),
		any(evt.Data()).(To),
		ID(evt.ID()),
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	)
}

// TryCast attempts to cast an event with data of type From to an event with
// data of type To. It returns the casted event and a boolean value indicating
// whether the casting was successful or not.
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

// Expand converts an Of[Data] event into an Evt[Data] event, preserving the
// event's properties. If the input event is already of type Evt[Data], it is
// returned directly. Otherwise, a new Evt[Data] event is created with the same
// properties as the input event.
func Expand[D any](evt Of[D]) Evt[D] {
	if evt, ok := evt.(Evt[D]); ok {
		return evt
	}
	return New(evt.Name(), evt.Data(), ID(evt.ID()), Time(evt.Time()), Aggregate(evt.Aggregate()))
}

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
