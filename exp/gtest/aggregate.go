package gtest

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
)

// ConstructorTest is a test for aggregate constructors. It checks if the
// constructor properly sets the AggregateID and calls the OnCreated hook, if
// provided, with the created aggregate.
type ConstructorTest[A aggregate.Aggregate] struct {
	Constructor func(uuid.UUID) A
	OnCreated   func(A) error
}

// ConstructorTestOption is a function that modifies a ConstructorTest for an
// Aggregate. It is used to customize the behavior of a ConstructorTest, such as
// providing a custom OnCreated hook function.
type ConstructorTestOption[A aggregate.Aggregate] func(*ConstructorTest[A])

// Created configures a ConstructorTest with a custom function to be called when
// an Aggregate is created, allowing for additional validation or setup steps.
// The provided function takes the created Aggregate as its argument and returns
// an error if any issues are encountered during execution.
func Created[A aggregate.Aggregate](fn func(A) error) ConstructorTestOption[A] {
	return func(test *ConstructorTest[A]) {
		test.OnCreated = fn
	}
}

// Constructor creates a new ConstructorTest with the specified constructor
// function and optional test options. It returns a pointer to the created
// ConstructorTest.
func Constructor[A aggregate.Aggregate](constructor func(uuid.UUID) A, opts ...ConstructorTestOption[A]) *ConstructorTest[A] {
	test := &ConstructorTest[A]{Constructor: constructor}
	for _, opt := range opts {
		opt(test)
	}
	return test
}

// Run executes the ConstructorTest, ensuring that the constructed aggregate has
// the correct UUID and, if provided, calls the OnCreated hook without errors.
// If any of these checks fail, an error is reported to the given testing.T.
func (test *ConstructorTest[A]) Run(t *testing.T) {
	t.Helper()

	id := uuid.New()
	a := test.Constructor(id)

	if pick.AggregateID(a) != id {
		t.Errorf("AggregateID should be %q; got %q", id, pick.AggregateID(a))
	}

	if test.OnCreated != nil {
		if err := test.OnCreated(a); err != nil {
			t.Errorf("OnCreated hook failed with %q", err)
		}
	}
}

// TransitionTest represents a test that checks whether an aggregate transitions
// to a specific event with the specified data. It can be used to ensure that an
// aggregate properly handles its internal state changes and produces the
// correct events with the expected data.
type TransitionTest[EventData any] struct {
	transitionTestConfig

	Event string
	Data  EventData

	isEqual func(EventData, EventData) bool
}

type transitionTestConfig struct {
	MatchCount uint
}

// TransitionTestOption is a function that modifies the behavior of a
// TransitionTest, such as configuring the number of times an event should be
// matched. It takes a transitionTestConfig struct and modifies its properties
// based on the desired configuration.
type TransitionTestOption func(*transitionTestConfig)

// Times is a TransitionTestOption that configures the number of times an event
// should match the expected data in a TransitionTest. It takes an unsigned
// integer argument representing the number of matches expected.
func Times(times uint) TransitionTestOption {
	return func(cfg *transitionTestConfig) {
		cfg.MatchCount = times
	}
}

// Once returns a TransitionTestOption that configures a TransitionTest to
// expect the specified event and data exactly once.
func Once() TransitionTestOption {
	return Times(1)
}

// Transition creates a new TransitionTest with the specified event name and
// data. It can be used to test if an aggregate transitions to the specified
// event with the provided data when running the Run method on a *testing.T
// instance.
func Transition[EventData comparable](event string, data EventData, opts ...TransitionTestOption) *TransitionTest[EventData] {
	return TransitionWithComparer(event, data, func(a, b EventData) bool { return a == b }, opts...)
}

// Comparable is an interface that provides a method for comparing two instances
// of the same type. The Equal method should return true if the two instances
// are considered equivalent, and false otherwise.
type Comparable[T any] interface {
	Equal(T) bool
}

// TransitionWithEqual creates a new TransitionTest with the specified event
// name and data, using the Equal method of the Comparable interface to compare
// event data. It is used for testing if an aggregate transitions to the
// specified event with the provided data, when running the Run method on a
// [*testing.T] instance. The comparison of event data is based on the Equal
// method, which should be implemented by the provided EventData type.
// TransitionTestOptions can be provided to customize the behavior of the
// TransitionTest.
func TransitionWithEqual[EventData Comparable[EventData]](event string, data EventData, opts ...TransitionTestOption) *TransitionTest[EventData] {
	return TransitionWithComparer(event, data, func(a, b EventData) bool { return a.Equal(b) }, opts...)
}

// TransitionWithComparer creates a new TransitionTest with the specified event
// name and data, using a custom comparison function to compare event data. It
// is used for testing if an aggregate transitions to the specified event with
// the provided data when running the Run method on a [*testing.T] instance.
// TransitionTestOptions can be provided to customize the behavior of the
// TransitionTest.
func TransitionWithComparer[EventData any](event string, data EventData, compare func(EventData, EventData) bool, opts ...TransitionTestOption) *TransitionTest[EventData] {
	test := TransitionTest[EventData]{
		Event:   event,
		Data:    data,
		isEqual: compare,
	}

	for _, opt := range opts {
		opt(&test.transitionTestConfig)
	}

	return &test
}

// Signal returns a new TransitionTest with the specified event name and no
// event data. It is used to test aggregate transitions for events without data.
func Signal(event string, opts ...TransitionTestOption) *TransitionTest[any] {
	return Transition[any](event, nil, opts...)
}

// Run tests whether an aggregate transitions to the specified event with the
// expected data. It reports an error if the aggregate does not transition to
// the specified event, or if the event data does not match the expected data.
func (test *TransitionTest[EventData]) Run(t *testing.T, a aggregate.Aggregate) {
	t.Helper()

	wantMatches := test.MatchCount
	if wantMatches == 0 {
		wantMatches = 1
	}

	var matches uint
	for _, evt := range a.AggregateChanges() {
		if evt.Name() != test.Event {
			continue
		}

		if test.testEquality(evt) == nil {
			matches++
		}
	}

	if matches != wantMatches {
		if wantMatches == 1 {
			t.Errorf("Aggregate %q should transition to %q with %T", pick.AggregateName(a), test.Event, test.Data)
			return
		}

		t.Errorf("Aggregate %q should transition to %q with %T %d times; got %d", pick.AggregateName(a), test.Event, test.Data, test.MatchCount, matches)
	}
}

func (test *TransitionTest[EventData]) testEquality(evt event.Event) error {
	if evt.Name() != test.Event {
		return fmt.Errorf("event name %q does not match expected event name %q", evt.Name(), test.Event)
	}

	cevt, ok := event.TryCast[EventData](evt)
	if !ok {
		return fmt.Errorf("event %q is not of type %T but of type %T", evt.Name(), test.Data, evt.Data())
	}

	var zero EventData
	if !test.isEqual(test.Data, zero) {
		data := cevt.Data()
		if !test.isEqual(test.Data, data) {
			return fmt.Errorf("event data %T does not match expected event data %T\n%s", data, test.Data, cmp.Diff(test.Data, data))
		}
	}

	return nil
}

// NonTransition represents an event that the aggregate should not transition
// to. It's used in testing to ensure that a specific event does not occur
// during the test run for a given aggregate.
type NonTransition string

// Run checks if the given aggregate a does not transition to the event
// specified by the NonTransition type. If it does, an error is reported with
// testing.T.
func (event NonTransition) Run(t *testing.T, a aggregate.Aggregate) {
	t.Helper()

	for _, evt := range a.AggregateChanges() {
		if evt.Name() == string(event) {
			t.Errorf("Aggregate %q should not transition to %q", pick.AggregateName(a), string(event))
		}
	}
}
