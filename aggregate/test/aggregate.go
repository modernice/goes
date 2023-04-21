package test

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	// FooAggregate is the aggregate name for Foo.
	FooAggregate = "foo"
)

// Foo is an example aggregate used for testing.
type Foo struct {
	testAggregate
}

// AggregateOption is an option for a test aggregate.
type AggregateOption func(*testAggregate)

type testAggregate struct {
	*aggregate.Base

	applyFuncs map[string]func(event.Event)
	trackFunc  func([]event.Event, func(...event.Event))
	commitFunc func(func())
}

// NewAggregate returns a new test aggregate.
func NewAggregate(name string, id uuid.UUID, opts ...AggregateOption) aggregate.Aggregate {
	a := &testAggregate{
		Base:       aggregate.New(name, id),
		applyFuncs: make(map[string]func(event.Event)),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// NewFoo returns a new Foo.
func NewFoo(id uuid.UUID, opts ...AggregateOption) *Foo {
	foo := Foo{
		testAggregate: testAggregate{
			Base:       aggregate.New("foo", id),
			applyFuncs: make(map[string]func(event.Event)),
		},
	}

	for _, opt := range opts {
		opt(&foo.testAggregate)
	}

	return &foo
}

// ApplyEventFunc returns an aggregateOption that allows users to intercept
// calls to a.ApplyEvent.
func ApplyEventFunc(eventName string, fn func(event.Event)) AggregateOption {
	return func(a *testAggregate) {
		a.applyFuncs[eventName] = fn
	}
}

// RecordChangeFunc returns an aggregateOption that allows users to intercept
// calls to a.RecordChange.
func RecordChangeFunc(fn func(changes []event.Event, track func(...event.Event))) AggregateOption {
	return func(a *testAggregate) {
		a.trackFunc = fn
	}
}

// CommitFunc returns an aggregateOption that allows users to intercept
// a.Commit calls. fn accepts a flush() function that can be called to
// actually flush the changes.
func CommitFunc(fn func(flush func())) AggregateOption {
	return func(a *testAggregate) {
		a.commitFunc = fn
	}
}

// ApplyEvent applies an
// [event](https://pkg.go.dev/github.com/modernice/goes/event#Event) to the
// testAggregate. If a function is registered for the event name of the event,
// that function will be called instead. If a function is registered for the
// empty event name, that function will be called instead.
func (a *testAggregate) ApplyEvent(evt event.Event) {
	if fn := a.applyFuncs[evt.Name()]; fn != nil {
		fn(evt)
		return
	}

	if fn := a.applyFuncs[""]; fn != nil {
		fn(evt)
	}
}

// RecordChange is a method that allows users to record changes in the state of
// an aggregate. It takes one or more events as arguments and stores them in the
// aggregate's uncommitted changes. If a RecordChangeFunc AggregateOption has
// been set, it will call that function instead of recording the changes
// directly. The RecordChangeFunc function accepts two arguments: a slice of
// changes and a track function that can be used to track additional changes.
func (a *testAggregate) RecordChange(changes ...event.Event) {
	if a.trackFunc == nil {
		a.recordChange(changes...)
		return
	}
	a.trackFunc(changes, a.recordChange)
}

func (a *testAggregate) recordChange(changes ...event.Event) {
	a.Base.RecordChange(changes...)
}

// Commit commits the changes recorded in the testAggregate. If a commitFunc
// AggregateOption was provided to NewAggregate or NewFoo, then that function
// will be called instead of the default commit function. The provided function
// should accept a flush function that can be called to actually flush the
// changes.
func (a *testAggregate) Commit() {
	if a.commitFunc == nil {
		a.commit()
		return
	}
	a.commitFunc(a.commit)
}

func (a *testAggregate) commit() {
	a.Base.Commit()
}
