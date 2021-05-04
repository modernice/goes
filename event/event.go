package event

//go:generate mockgen -source=event.go -destination=./mocks/event.go

import (
	"sort"
	"time"

	"github.com/google/uuid"
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
	Time() time.Time
	// Data returns the Event Data.
	Data() Data

	// AggregateName returns the name of the Aggregate the Event belongs to.
	AggregateName() string
	// AggregateID returns the UUID of the Aggregate the Event belongs to.
	AggregateID() uuid.UUID
	// AggregateVersion returns the version of the Aggregate the Event is equal to.
	AggregateVersion() int
}

// Data is the data of an Event.
type Data interface{}

// Option is an Event option.
type Option func(*event)

type event struct {
	id               uuid.UUID
	name             string
	time             time.Time
	data             Data
	aggregateName    string
	aggregateID      uuid.UUID
	aggregateVersion int
}

// New creates an Event with the given name and Data. A UUID is generated for
// the Event and its time is set to time.Now().
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
		time: time.Now(),
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
func Time(t time.Time) Option {
	return func(evt *event) {
		evt.time = t
	}
}

// Aggregate returns an Option that links an Event to an Aggregate.
func Aggregate(name string, id uuid.UUID, version int) Option {
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
	v := prev.AggregateVersion()
	if prev.AggregateID() != uuid.Nil {
		v++
	}
	return Aggregate(prev.AggregateName(), prev.AggregateID(), v)
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
	for _, evt := range events[1:] {
		if (evt == nil && first != nil) || (evt != nil && first == nil) {
			return false
		}

		if !(evt.ID() == first.ID() &&
			evt.Name() == first.Name() &&
			evt.Time().Equal(first.Time()) &&
			evt.Data() == first.Data() &&
			evt.AggregateID() == first.AggregateID() &&
			evt.AggregateName() == first.AggregateName() &&
			evt.AggregateVersion() == first.AggregateVersion()) {
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

func (evt event) ID() uuid.UUID {
	return evt.id
}

func (evt event) Name() string {
	return evt.name
}

func (evt event) Time() time.Time {
	return evt.time
}

func (evt event) Data() Data {
	return evt.data
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
