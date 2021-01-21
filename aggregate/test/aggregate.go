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

// Foo is an example Aggregate used for testing.
type Foo struct {
	testAggregate
}

// AggregateOption is an option for a test Aggregate.
type AggregateOption func(*testAggregate)

type testAggregate struct {
	aggregate.Aggregate
	applyFuncs map[string]func(event.Event)
	flushFunc  func(func())
}

// NewFoo returns a new Foo.
func NewFoo(id uuid.UUID, opts ...AggregateOption) *Foo {
	foo := Foo{
		testAggregate: testAggregate{
			Aggregate:  aggregate.New("foo", id),
			applyFuncs: make(map[string]func(event.Event)),
		},
	}

	for _, opt := range opts {
		opt(&foo.testAggregate)
	}

	return &foo
}

// ApplyEventFunc returns an AggregateOption that allows users to intercept
// a.ApplyEvent calls.
func ApplyEventFunc(eventName string, fn func(event.Event)) AggregateOption {
	return func(a *testAggregate) {
		a.applyFuncs[eventName] = fn
	}
}

// FlushChangesFunc returns an AggregateOption that allows users to intercept
// a.FlushChanges calls. fn accepts a flush() function that can be called to
// actually flush the changes.
func FlushChangesFunc(fn func(flush func())) AggregateOption {
	return func(a *testAggregate) {
		a.flushFunc = fn
	}
}

// // ApplyEvent applies the Event evt if an apply function was provided with
// // ApplyEventFunc.
// func (foo *Foo) ApplyEvent(evt event.Event) {
// 	foo.base.ApplyEvent(evt)
// }

func (a *testAggregate) ApplyEvent(evt event.Event) {
	if fn := a.applyFuncs[evt.Name()]; fn != nil {
		fn(evt)
	}
}

func (a *testAggregate) FlushChanges() {
	if a.flushFunc == nil {
		a.flushChanges()
		return
	}
	a.flushFunc(a.flushChanges)
}

func (a *testAggregate) flushChanges() {
	a.Aggregate.FlushChanges()
}
