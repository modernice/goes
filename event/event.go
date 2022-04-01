package event

//go:generate mockgen -source=event.go -destination=./mocks/event.go

import (
	"sort"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/internal/xtime"
)

// Event is any event.
type Event = Of[any]

// An event describes something that has happened in the application or
// specifically something that has happened to an aggregate in the application.
//
// Publish & Subscribe
//
// An event can be published through a Bus and sent to subscribers of Events
// with the same name.
//
// Example (publish):
// 	var b event.Bus
// 	evt := event.New("foo", someData{})
// 	err := b.Publish(context.TODO(), evt)
// 	// handle err
//
// Example (subscribe):
// 	var b event.Bus
// 	res, errs, err := b.Subscribe(context.TODO(), "foo")
// 	// handle err
//	err := streams.Walk(context.TODO(), func(e event.Event) {
// 	    log.Println(fmt.Sprintf("Received %q event: %v", e.Name(), e))
//	}, res, errs)
//	// handle err
type Of[Data any] interface {
	// ID returns the unique id of the event.
	ID() uuid.UUID
	// Name returns the name of the event.
	Name() string
	// Time returns the time of the event.
	Time() stdtime.Time
	// Data returns the event Data.
	Data() Data

	// Aggregate returns the id, name and version of the aggregate that the
	// event belongs to. aggregate should return zero values if the event is not
	// an aggregate event.
	Aggregate() (id uuid.UUID, name string, version int)
}

// Option is an event option.
type Option func(*Evt[any])

// Evt is the event implementation.
type Evt[D any] struct {
	D Data[D]
}

// Data can be used to provide the data that is needed to implement the event
// interface. E embeds Data and provides the methods that return the data in
// Data.
type Data[D any] struct {
	ID               uuid.UUID
	Name             string
	Time             stdtime.Time
	Data             D
	AggregateName    string
	AggregateID      uuid.UUID
	AggregateVersion int
}

// New creates an event with the given name and Data. A UUID is generated for
// the event and its time is set to xtime.Now().
//
// Provide Options to override or add data to the event:
//	ID(uuid.UUID): Use a custom UUID
//	Time(time.Time): Use a custom Time
//	Aggregate(string, uuid.UUID, int): Add aggregate data
//	Previous(event.Event): Set aggregate data based on previous event
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

// ID returns an Option that overrides the auto-generated UUID of an event.
func ID(id uuid.UUID) Option {
	return func(evt *Evt[any]) {
		evt.D.ID = id
	}
}

// Time returns an Option that overrides the auto-generated timestamp of an event.
func Time(t stdtime.Time) Option {
	return func(evt *Evt[any]) {
		evt.D.Time = t
	}
}

// Aggregate returns an Option that links an event to an aggregate.
func Aggregate(id uuid.UUID, name string, version int) Option {
	return func(evt *Evt[any]) {
		evt.D.AggregateName = name
		evt.D.AggregateID = id
		evt.D.AggregateVersion = version
	}
}

// Previous returns an Option that adds aggregate data to an event. If prev
// provides non-nil aggregate data (id, name & version), the returned Option
// adds those to the new event with its version increased by 1.
func Previous[Data any](prev Of[Data]) Option {
	id, name, v := prev.Aggregate()
	if id != uuid.Nil {
		v++
	}
	return Aggregate(id, name, v)
}

// Equal compares events and determines if they're equal. It works exactly like
// a normal "==" comparison except for the Time field which is being compared by
// calling a.Time().Equal(b.Time()) for the two events a and b that are being
// compared.
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

// Sort sorts events and returns the sorted events.
func Sort[D any, Events ~[]Of[D]](events Events, sort Sorting, dir SortDirection) Events {
	return SortMulti(events, SortOptions{Sort: sort, Dir: dir})
}

// SortMulti sorts events by multiple fields and returns the sorted events.
func SortMulti[D any, Events ~[]Of[D]](events Events, sorts ...SortOptions) Events {
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

func (evt Evt[D]) ID() uuid.UUID {
	return evt.D.ID
}

func (evt Evt[D]) Name() string {
	return evt.D.Name
}

func (evt Evt[D]) Time() stdtime.Time {
	return evt.D.Time
}

func (evt Evt[D]) Data() D {
	return evt.D.Data
}

func (evt Evt[D]) Aggregate() (uuid.UUID, string, int) {
	return evt.D.AggregateID, evt.D.AggregateName, evt.D.AggregateVersion
}

// Any returns the event with its type paramter set to `any`.
func (evt Evt[D]) Any() Evt[any] {
	return Any[D](evt)
}

// Event returns the event as an interface.
func (evt Evt[D]) Event() Of[D] {
	return evt
}

// Any casts the event data of the given event to `any`.
func Any[Data any](evt Of[Data]) Evt[any] {
	return Cast[any](evt)
}

// Cast casts the type paramater of given event to the type `To`. Cast panics if
// the event data is not a `To`.
//
// Use TryCast to test if the event data can be casted to `To`.
func Cast[To, From any](evt Of[From]) Evt[To] {
	return New(
		evt.Name(),
		any(evt.Data()).(To),
		ID(evt.ID()),
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	)
}

// TryCast casts the type paramater of given event to the type `To`. Cast
// returns false if the event data cannot be casted to `To`.
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

// Expand returns an Evt struct from an event interface.
func Expand[D any](evt Of[D]) Evt[D] {
	if evt, ok := evt.(Evt[D]); ok {
		return evt
	}
	return New(evt.Name(), evt.Data(), ID(evt.ID()), Time(evt.Time()), Aggregate(evt.Aggregate()))
}

// Test tests the event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
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

func timesContains(times []stdtime.Time, t stdtime.Time) bool {
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

func testTimeRanges(ranges []time.Range, t stdtime.Time) bool {
	for _, r := range ranges {
		if r.Includes(t) {
			return true
		}
	}
	return false
}

func testMinTimes(min stdtime.Time, t stdtime.Time) bool {
	if t.Equal(min) || t.After(min) {
		return true
	}
	return false
}

func testMaxTimes(max stdtime.Time, t stdtime.Time) bool {
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
