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

var _ aggregate.Aggregate[any] = (*Foo)(nil)

// Foo is an example Aggregate used for testing.
type Foo struct {
	testAggregate
}

// AggregateOption is an option for a test Aggregate.
type AggregateOption func(*testAggregate)

type testAggregate struct {
	*aggregate.Base[any]

	applyFuncs map[string]func(event.Event[any])
	trackFunc  func([]event.Event[any], func(...event.Event[any]))
	commitFunc func(func())
}

// NewAggregate returns a new test aggregate.
func NewAggregate(name string, id uuid.UUID, opts ...AggregateOption) aggregate.Aggregate[any] {
	a := &testAggregate{
		Base:       aggregate.New[any](name, id),
		applyFuncs: make(map[string]func(event.Event[any])),
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
			Base:       aggregate.New[any]("foo", id),
			applyFuncs: make(map[string]func(event.Event[any])),
		},
	}

	for _, opt := range opts {
		opt(&foo.testAggregate)
	}

	return &foo
}

// ApplyEventFunc returns an AggregateOption that allows users to intercept
// calls to a.ApplyEvent.
func ApplyEventFunc(eventName string, fn func(event.Event[any])) AggregateOption {
	return func(a *testAggregate) {
		a.applyFuncs[eventName] = fn
	}
}

// TrackChangeFunc returns an AggregateOption that allows users to intercept
// calls to a.TrackChange.
func TrackChangeFunc(fn func(changes []event.Event[any], track func(...event.Event[any]))) AggregateOption {
	return func(a *testAggregate) {
		a.trackFunc = fn
	}
}

// CommitFunc returns an AggregateOption that allows users to intercept
// a.Commit calls. fn accepts a flush() function that can be called to
// actually flush the changes.
func CommitFunc(fn func(flush func())) AggregateOption {
	return func(a *testAggregate) {
		a.commitFunc = fn
	}
}

func (a *testAggregate) ApplyEvent(evt event.Event[any]) {
	if fn := a.applyFuncs[evt.Name()]; fn != nil {
		fn(evt)
		return
	}

	if fn := a.applyFuncs[""]; fn != nil {
		fn(evt)
	}
}

func (a *testAggregate) TrackChange(changes ...event.Event[any]) {
	if a.trackFunc == nil {
		a.trackChange(changes...)
		return
	}
	a.trackFunc(changes, a.trackChange)
}

func (a *testAggregate) trackChange(changes ...event.Event[any]) {
	a.Base.TrackChange(changes...)
}

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
