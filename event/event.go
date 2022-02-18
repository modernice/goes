package event

//go:generate mockgen -source=event.go -destination=./mocks/event.go

import (
	"sort"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/internal/xtime"
)

// Event is any event.
type Event = Of[any, uuid.UUID]

// An Event describes something that has happened in the application or
// specifically something that has happened to an Aggregate in the application.
//
// Publish & Subscribe
//
// An Event can be published through a Bus and sent to subscribers of Events
// with the same name.
//
// Example (publish):
// 	var b event.Bus
// 	evt := event.New(uuid.New(), "foo", someData{})
// 	err := b.Publish(context.TODO(), evt)
// 	// handle err
//
// Example (subscribe):
// 	var b event.Bus
// 	res, errs, err := b.Subscribe(context.TODO(), "foo")
// 	// handle err
//	err := streams.Walk(context.TODO(), func(e event.Of[any, uuid.UUID]) {
// 	    log.Println(fmt.Sprintf("Received %q event: %v", e.Name(), e))
//	}, res, errs)
//	// handle err
type Of[Data any, ID goes.ID] interface {
	// ID returns the unique id of the Event.
	ID() ID
	// Name returns the name of the Event.
	Name() string
	// Time returns the time of the Event.
	Time() stdtime.Time
	// Data returns the Event Data.
	Data() Data

	// Aggregate returns the id, name and version of the aggregate that the
	// event belongs to. Aggregate should return zero values if the event is not
	// an aggregate event.
	Aggregate() (id ID, name string, version int)
}

// Option is an event option.
type Option func(*options)

type options struct {
	id               any
	name             string
	time             stdtime.Time
	data             any
	aggregateName    string
	aggregateID      any
	aggregateVersion int
}

// E implements Event.
type E[D any, ID goes.ID] struct {
	D Data[D, ID]
}

// Data can be used to provide the data that is needed to implement the Event
// interface. E embeds Data and provides the methods that return the data in
// Data.
type Data[D any, ID goes.ID] struct {
	ID               ID
	Name             string
	Time             stdtime.Time
	Data             D
	AggregateName    string
	AggregateID      ID
	AggregateVersion int
}

// New creates an Event with the given name and Data. A UUID is generated for
// the Event and its time is set to xtime.Now().
//
// Provide Options to override or add data to the Event:
//	ID(uuid.UUID): Use a custom UUID
//	Time(time.Time): Use a custom Time
//	Aggregate(string, uuid.UUID, int): Add Aggregate data
//	Previous(event.Of[any, uuid.UUID]): Set Aggregate data based on previous Event
func New[D any, ID goes.ID](id ID, name string, data D, opts ...Option) E[D, ID] {
	eopts := options{
		id:   id,
		name: name,
		time: xtime.Now(),
		data: data,
	}
	for _, opt := range opts {
		opt(&eopts)
	}

	if eopts.id != nil {
		id, _ = eopts.id.(ID)
	}

	var aggregateID ID
	if eopts.aggregateID != nil {
		aggregateID, _ = eopts.aggregateID.(ID)
	}

	return E[D, ID]{
		D: Data[D, ID]{
			ID:               id,
			Name:             eopts.name,
			Time:             eopts.time,
			Data:             data,
			AggregateName:    eopts.aggregateName,
			AggregateID:      aggregateID,
			AggregateVersion: eopts.aggregateVersion,
		},
	}
}

// ID returns an Option that overrides the auto-generated UUID of an event.
func ID[T comparable](id T) Option {
	return func(opts *options) {
		opts.id = id
	}
}

// Time returns an Option that overrides the auto-generated timestamp of an Event.
func Time(t stdtime.Time) Option {
	return func(opts *options) {
		opts.time = t
	}
}

// Aggregate returns an Option that links an Event to an Aggregate.
func Aggregate[ID goes.ID](id ID, name string, version int) Option {
	return func(opts *options) {
		opts.aggregateName = name
		opts.aggregateID = id
		opts.aggregateVersion = version
	}
}

// Previous returns an Option that adds Aggregate data to an Event. If prev
// provides non-nil aggregate data (id, name & version), the returned Option
// adds those to the new event with its version increased by 1.
func Previous[Data any, ID goes.ID](prev Of[Data, ID]) Option {
	var zero ID
	id, name, v := prev.Aggregate()
	if id != zero {
		v++
	}
	return Aggregate(id, name, v)
}

// Equal compares events and determines if they're equal. It works exactly like
// a normal "==" comparison except for the Time field which is being compared by
// calling a.Time().Equal(b.Time()) for the two Events a and b that are being
// compared.
func Equal[D comparable, ID goes.ID](events ...Of[D, ID]) bool {
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
func Sort[D any, ID goes.ID, Events ~[]Of[D, ID]](events Events, sort Sorting, dir SortDirection) Events {
	return SortMulti[D, ID, Events](events, SortOptions{Sort: sort, Dir: dir})
}

// SortMulti sorts events by multiple fields and returns the sorted events.
func SortMulti[D any, ID goes.ID, Events ~[]Of[D, ID]](events Events, sorts ...SortOptions) Events {
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

func (evt E[D, ID]) ID() ID {
	return evt.D.ID
}

func (evt E[D, ID]) Name() string {
	return evt.D.Name
}

func (evt E[D, ID]) Time() stdtime.Time {
	return evt.D.Time
}

func (evt E[D, ID]) Data() D {
	return evt.D.Data
}

func (evt E[D, ID]) Aggregate() (ID, string, int) {
	return evt.D.AggregateID, evt.D.AggregateName, evt.D.AggregateVersion
}

// Any returns the event with its data type set to `any`.
func (evt E[D, ID]) Any() E[any, ID] {
	return ToAny[D, ID](evt)
}

// Event returns the event as an interface.
func (evt E[D, ID]) Event() Of[D, ID] {
	return evt
}

// ToAny casts the type parameter of the given event to `any` and returns the
// re-typed event.
func ToAny[D any, ID goes.ID](evt Of[D, ID]) E[any, ID] {
	return Cast[any](evt)
}

func Many[D any, ID goes.ID, Events ~[]Of[D, ID]](events Events) []E[any, ID] {
	out := make([]E[any, ID], len(events))
	for i, evt := range events {
		out[i] = ToAny(evt)
	}
	return out
}

// Cast casts the type paramater of given event to the type `To` and returns
// the re-typed event. Cast panics if the event data cannot be casted to `To`.
//
// Use TryCast to test if the event data can be casted to `To`.
func Cast[To, From any, IDType goes.ID](evt Of[From, IDType]) E[To, IDType] {
	return New(
		evt.ID(),
		evt.Name(),
		any(evt.Data()).(To),
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	)
}

// TryCast casts the type paramater of given event to the type `To` and returns
// the re-typed event. Cast returns false if the event data cannot be casted to
// `To`.
func TryCast[To, From any, IDType goes.ID](evt Of[From, IDType]) (E[To, IDType], bool) {
	data, ok := any(evt.Data()).(To)
	if !ok {
		return E[To, IDType]{}, false
	}
	return New(
		evt.ID(),
		evt.Name(),
		data,
		Time(evt.Time()),
		Aggregate(evt.Aggregate()),
	), true
}

func CastMany[To, From any, ID goes.ID, FromEvents ~[]Of[From, ID]](events FromEvents) []E[To, ID] {
	out := make([]E[To, ID], len(events))
	for i, evt := range events {
		out[i] = Cast[To](evt)
	}
	return out
}

func TryCastMany[To, From any, ID goes.ID, FromEvents ~[]Of[From, ID]](events FromEvents) ([]Of[To, ID], bool) {
	out := make([]Of[To, ID], len(events))
	for i, evt := range events {
		var ok bool
		if out[i], ok = TryCast[To](evt); !ok {
			return out, ok
		}
	}
	return out, true
}

// AnyID returns the event with its id wrapped in a goes.AID.
func AnyID[D any, ID goes.ID](evt Of[D, ID]) E[D, goes.AID] {
	ex := Expand(evt)
	return E[D, goes.AID]{
		D: Data[D, goes.AID]{
			ID:               goes.AnyID(ex.ID()),
			Name:             ex.Name(),
			Time:             ex.Time(),
			Data:             ex.Data(),
			AggregateName:    ex.D.AggregateName,
			AggregateID:      goes.AnyID(ex.D.AggregateID),
			AggregateVersion: ex.D.AggregateVersion,
		},
	}
}

func MapID[To, From goes.ID, D any](evt Of[D, From], mapper func(From) To) E[D, To] {
	ex := Expand(evt)

	var zero From
	var id, aggregateID To

	if eid := ex.ID(); eid != zero {
		id = mapper(eid)
	}

	if ex.D.AggregateID != zero {
		aggregateID = mapper(ex.D.AggregateID)
	}

	return E[D, To]{
		D: Data[D, To]{
			ID:               id,
			Name:             ex.Name(),
			Time:             ex.Time(),
			Data:             ex.Data(),
			AggregateName:    ex.D.AggregateName,
			AggregateID:      aggregateID,
			AggregateVersion: ex.D.AggregateVersion,
		},
	}
}

// Expand expands an interfaced event to a struct.
func Expand[D any, IDType goes.ID](evt Of[D, IDType]) E[D, IDType] {
	if evt, ok := evt.(E[D, IDType]); ok {
		return evt
	}
	return New(evt.ID(), evt.Name(), evt.Data(), Time(evt.Time()), Aggregate(evt.Aggregate()))
}

// Test tests the Event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
func Test[D any, ID goes.ID](q QueryOf[ID], evt Of[D, ID]) bool {
	if q == nil {
		return true
	}

	var zeroID ID

	if names := q.Names(); len(names) > 0 &&
		!stringsContains(names, evt.Name()) {
		return false
	}

	if ids := q.IDs(); len(ids) > 0 && !idsContains(ids, evt.ID()) {
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
		!idsContains(ids, id) {
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
				(aggregate.ID == zeroID || aggregate.ID == id) {
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

func idsContains[T comparable](ids []T, id T) bool {
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
