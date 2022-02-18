package test

import (
	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const (
	// FooAggregate is the aggregate name for Foo.
	FooAggregate = "foo"
)

// Foo is an example Aggregate used for testing.
type Foo[ID goes.ID] struct {
	testAggregate[ID]
}

// AggregateOption is an option for a test Aggregate.
type AggregateOption[ID goes.ID] func(*testAggregate[ID])

type testAggregate[ID goes.ID] struct {
	*aggregate.Base[ID]

	applyFuncs map[string]func(event.Of[any, ID])
	trackFunc  func([]event.Of[any, ID], func(...event.Of[any, ID]))
	commitFunc func(func())
}

// NewAggregate returns a new test aggregate.
func NewAggregate[ID goes.ID](name string, id ID, opts ...AggregateOption[ID]) aggregate.AggregateOf[ID] {
	a := &testAggregate[ID]{
		Base:       aggregate.New(name, id),
		applyFuncs: make(map[string]func(event.Of[any, ID])),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// NewFoo returns a new Foo.
func NewFoo[ID goes.ID](id ID, opts ...AggregateOption[ID]) *Foo[ID] {
	foo := Foo[ID]{
		testAggregate: testAggregate[ID]{
			Base:       aggregate.New("foo", id),
			applyFuncs: make(map[string]func(event.Of[any, ID])),
		},
	}

	for _, opt := range opts {
		opt(&foo.testAggregate)
	}

	return &foo
}

// ApplyEventFunc returns an AggregateOption that allows users to intercept
// calls to a.ApplyEvent.
func ApplyEventFunc[ID goes.ID](eventName string, fn func(event.Of[any, ID])) AggregateOption[ID] {
	return func(a *testAggregate[ID]) {
		a.applyFuncs[eventName] = fn
	}
}

// TrackChangeFunc returns an AggregateOption that allows users to intercept
// calls to a.TrackChange.
func TrackChangeFunc[ID goes.ID](fn func(changes []event.Of[any, ID], track func(...event.Of[any, ID]))) AggregateOption[ID] {
	return func(a *testAggregate[ID]) {
		a.trackFunc = fn
	}
}

// CommitFunc returns an AggregateOption that allows users to intercept
// a.Commit calls. fn accepts a flush() function that can be called to
// actually flush the changes.
func CommitFunc[ID goes.ID](fn func(flush func())) AggregateOption[ID] {
	return func(a *testAggregate[ID]) {
		a.commitFunc = fn
	}
}

func (a *testAggregate[ID]) ApplyEvent(evt event.Of[any, ID]) {
	if fn := a.applyFuncs[evt.Name()]; fn != nil {
		fn(evt)
		return
	}

	if fn := a.applyFuncs[""]; fn != nil {
		fn(evt)
	}
}

func (a *testAggregate[ID]) TrackChange(changes ...event.Of[any, ID]) {
	if a.trackFunc == nil {
		a.trackChange(changes...)
		return
	}
	a.trackFunc(changes, a.trackChange)
}

func (a *testAggregate[ID]) trackChange(changes ...event.Of[any, ID]) {
	a.Base.TrackChange(changes...)
}

func (a *testAggregate[ID]) Commit() {
	if a.commitFunc == nil {
		a.commit()
		return
	}
	a.commitFunc(a.commit)
}

func (a *testAggregate[ID]) commit() {
	a.Base.Commit()
}
