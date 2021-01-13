package event

import (
	"time"

	"github.com/google/uuid"
)

// An Event is an event that happened in the application. It provides at least
// an id, name, timestamp and Data. An event that belongs to an aggregate
// additionaly provides the aggregate name, aggregate id and aggrgate version.
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
	// AggregateID returns the id of the Aggregate the Event belongs to.
	AggregateID() uuid.UUID
	// AggregateVersion returns the version of the Aggregate the Event belongs to.
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

// New returns an Event with the specified name and Data. New automatically
// generates a UUID for the Event and sets its timestamp to time.Now().
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

// Time returns an Option that sets the timestamp of an Event.
func Time(t time.Time) Option {
	return func(evt *event) {
		evt.time = t
	}
}

// Aggregate returns an Option that adds Aggregate information to an Event.
func Aggregate(name string, id uuid.UUID, version int) Option {
	return func(evt *event) {
		evt.aggregateName = name
		evt.aggregateID = id
		evt.aggregateVersion = version
	}
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
