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
// 	evt := event.New("foo", someData{})
// 	err := b.Publish(context.TODO(), evt)
// 	// handle err
//
// Example (subscribe):
// 	var b event.Bus
// 	res, errs, err := b.Subscribe(context.TODO(), "foo")
// 	// handle err
//	err := event.Walk(context.TODO(), func(e event.Event) {
// 	    log.Println(fmt.Sprintf("Received %q event: %v", e.Name(), e))
//	}, res, errs)
//	// handle err
type Event interface {
	// ID returns the unique id of the Event.
	ID() uuid.UUID
	// Name returns the name of the Event.
	Name() string
	// Time returns the time of the Event.
	Time() stdtime.Time
	// Data returns the Event Data.
	Data() Data

	// Aggregate returns the id, name and version of the aggregate that the
	// event belongs to. Aggregate should return zero values if the event is not
	// an aggregate event.
	Aggregate() (id uuid.UUID, name string, version int)
}

// Data is the data of an Event.
type Data interface{}

// Option is an Event option.
type Option func(*event)

type event struct {
	id               uuid.UUID
	name             string
	time             stdtime.Time
	data             Data
	aggregateName    string
	aggregateID      uuid.UUID
	aggregateVersion int
}

// New creates an Event with the given name and Data. A UUID is generated for
// the Event and its time is set to xtime.Now().
//
// Provide Options to override or add data to the Event:
//	ID(uuid.UUID): Use a custom UUID
//	Time(time.Time): Use a custom Time
//	Aggregate(string, uuid.UUID, int): Add Aggregate data
//	Previous(event.Event): Set Aggregate data based on previous Event
func New(name string, data Data, opts ...Option) Event {
	evt := event{
		id:   uuid.New(),
		name: name,
		time: xtime.Now(),
		data: data,
	}
	for _, opt := range opts {
		opt(&evt)
	}
	return evt
}

// ID returns an Option that overrides the auto-generated UUID of an Event.
func ID(id uuid.UUID) Option {
	return func(evt *event) {
		evt.id = id
	}
}

// Time returns an Option that overrides the auto-generated timestamp of an Event.
func Time(t stdtime.Time) Option {
	return func(evt *event) {
		evt.time = t
	}
}

// Aggregate returns an Option that links an Event to an Aggregate.
func Aggregate(id uuid.UUID, name string, version int) Option {
	return func(evt *event) {
		evt.aggregateName = name
		evt.aggregateID = id
		evt.aggregateVersion = version
	}
}

// Previous returns an Option that adds Aggregate data to an Event. If prev
// provides non-nil Aggregate data (UUID, name & version), the returned Option
// adds those to the new Event with its version increased by 1.
func Previous(prev Event) Option {
	id, name, v := prev.Aggregate()
	if id != uuid.Nil {
		v++
	}
	return Aggregate(id, name, v)
}

// Equal compares events and determines if they're equal. It works exactly like
// a normal "==" comparison except for the Time field which is being compared by
// calling a.Time().Equal(b.Time()) for the two Events a and b that are being
// compared.
func Equal(events ...Event) bool {
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
func Sort(events []Event, sort Sorting, dir SortDirection) []Event {
	return SortMulti(events, SortOptions{Sort: sort, Dir: dir})
}

// SortMulti sorts events by multiple fields and returns the sorted events.
func SortMulti(events []Event, sorts ...SortOptions) []Event {
	sorted := make([]Event, len(events))
	copy(sorted, events)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			cmp := opts.Sort.Compare(sorted[i], sorted[j])
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}

// AggregateID returns the AggregateID of the given event.
func AggregateID(evt Event) uuid.UUID {
	id, _, _ := evt.Aggregate()
	return id
}

// AggregateName returns the AggregateName of the given event.
func AggregateName(evt Event) string {
	_, name, _ := evt.Aggregate()
	return name
}

// AggregateVersion returns the AggregateVersion of the given event.
func AggregateVersion(evt Event) int {
	_, _, v := evt.Aggregate()
	return v
}

func (evt event) ID() uuid.UUID {
	return evt.id
}

func (evt event) Name() string {
	return evt.name
}

func (evt event) Time() stdtime.Time {
	return evt.time
}

func (evt event) Data() Data {
	return evt.data
}

func (evt event) Aggregate() (uuid.UUID, string, int) {
	return evt.aggregateID, evt.aggregateName, evt.aggregateVersion
}

func (evt event) AggregateName() string {
	return evt.aggregateName
}

func (evt event) AggregateID() uuid.UUID {
	return evt.aggregateID
}

func (evt event) AggregateVersion() int {
	return evt.aggregateVersion
}

// Test tests the Event evt against the Query q and returns true if q should
// include evt in its results. Test can be used by in-memory event.Store
// implementations to filter events based on the query.
func Test(q Query, evt Event) bool {
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
