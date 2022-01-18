package test

import (
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

var (
	// ExampleID is a UUID that can be used in tests.
	ExampleID = uuid.New()
)

// NewAggregate tests the New function of an aggregate to check if the returned
// aggregate provides the correct AggregateName and AggregateID.
//
// Example:
//	type Foo struct {
//		*aggregate.Base
//	}
//
//	func NewFoo() *Foo {
//		return &Foo{Base: aggregate.New()}
//	}
//
//	func TestNewFoo() {
//		test[E any].NewAggregate(func(id uuid.UUID) aggregate.Aggregate {
//			return NewFoo(id)
//		})
//	}
func NewAggregate[A aggregate.Aggregate](t TestingT, newFunc func(uuid.UUID) A, expectedName string) {
	a := newFunc(ExampleID)

	id, name, _ := a.Aggregate()

	if name != expectedName {
		t.Fatal(fmt.Sprintf("AggregateName() should return %q; got %q", expectedName, name))
	}

	if id != ExampleID {
		t.Fatal(fmt.Sprintf("AggregateID() should return %q; got %q", ExampleID, id))
	}
}

// ExpectedChangeError is returned by the `Change` testing helper when the
// testd Aggregate doesn't have the required change.
type ExpectedChangeError[E comparable] struct {
	// EventName is the name of the tested change.
	EventName string

	// Matches is the number of changes that matched.
	Matches int

	cfg          changeConfig[E]
	mismatchData []E
}

func (err ExpectedChangeError[E]) Error() string {
	var eventDataSuffix string
	var zero E

	if err.cfg.eventData != zero {
		eventDataSuffix = fmt.Sprintf(" with event data\n\n%v\n\n", err.cfg.eventData)

		if l := len(err.mismatchData); l > 0 {
			eventDataSuffix = fmt.Sprintf("%sbut got %d change(s) with event data\n\n%v\n\n", eventDataSuffix, l, err.mismatchData)
		}
	}

	if err.cfg.atLeast > 0 && err.Matches < err.cfg.atLeast {
		return fmt.Sprintf("expected at least %d %q changes%s; got %d", err.cfg.atLeast, err.EventName, eventDataSuffix, err.Matches)
	}

	if err.cfg.atMost > 0 && err.Matches > err.cfg.atMost {
		return fmt.Sprintf("expected at most %d %q changes%s; got %d", err.cfg.atMost, err.EventName, eventDataSuffix, err.Matches)
	}

	if err.cfg.exactly > 0 && err.Matches != err.cfg.exactly {
		return fmt.Sprintf("expected exactly %d %q changes%s; got %d", err.cfg.exactly, err.EventName, eventDataSuffix, err.Matches)
	}

	return fmt.Sprintf("expected %q change%s", err.EventName, eventDataSuffix)
}

// UnexpectedChangeError is returned by the `NoChange` testing helper when the
// testd Aggregate does have an unwanted change.
type UnexpectedChangeError struct {
	// EventName is the name of the tested change.
	EventName string
}

func (err UnexpectedChangeError) Error() string {
	return fmt.Sprintf("unexpected %q change", err.EventName)
}

// ChangeOption is an option for the `Change` testing helper.
type ChangeOption[E any] func(*changeConfig[E])

type changeConfig[E any] struct {
	eventData E
	changeConfigValues
}

type changeConfigValues struct {
	atLeast int
	atMost  int
	exactly int
}

// EventData returns a ChangeOption that also tests the event data of
// changes instead of just the event name.
func EventData[E any](d E) ChangeOption[E] {
	return func(cfg *changeConfig[E]) {
		cfg.eventData = d
	}
}

// Deprecated: Use EventData instead.
func WithEventData[E any](d E) ChangeOption[E] {
	return EventData(d)
}

// AtLeast returns a ChangeOption that requires an Aggregate to have a change at
// least as many times as provided.
//
// AtLeast has no effect when used in `NoChange`.
func AtLeast[E any](times int) ChangeOption[E] {
	return func(cfg *changeConfig[E]) {
		cfg.atLeast = times
	}
}

// AtMost returns a ChangeOption that requires an Aggregate to have a change at
// most as many times as provided.
//
// AtMost has no effect when used in `NoChange`.
func AtMost[E any](times int) ChangeOption[E] {
	return func(cfg *changeConfig[E]) {
		cfg.atMost = times
	}
}

// Exactly returns a ChangeOption that requires an Aggregate to have a change
// exactly as many times as provided.
//
// Exactly has no effect when used in `NoChange`.
func Exactly[E any](times int) ChangeOption[E] {
	return func(cfg *changeConfig[E]) {
		cfg.exactly = times
	}
}

// Change tests an Aggregate for a change. The Aggregate must have an
// uncommitted change with the specified event name.
func Change[E comparable](t TestingT, a aggregate.Aggregate, eventName string, opts ...ChangeOption[E]) {
	var cfg changeConfig[E]
	for _, opt := range opts {
		opt(&cfg)
	}

	var matches int
	var mismatchData []E
	var zero E

	for _, change := range a.AggregateChanges() {
		if change.Name() != eventName {
			continue
		}

		casted, ok := event.TryCast[E](change)
		if !ok {
			t.Fatal(fmt.Errorf(
				"cannot cast %T to %T. either provide %T to the test.EventData option or provide the correct event name for %T.",
				change.Data(), casted.Data(), casted.Data(), casted.Data(),
			))
			continue
		}

		if cfg.eventData != zero && !reflect.DeepEqual(cfg.eventData, change.Data()) {
			mismatchData = append(mismatchData, casted.Data())
			continue
		}

		matches++
	}

	if cfg.atLeast > 0 && matches < cfg.atLeast {
		t.Fatal(&ExpectedChangeError[E]{
			EventName: eventName,
			Matches:   matches,
			cfg:       cfg,
		})
		return
	}

	if cfg.atMost > 0 && matches > cfg.atMost {
		t.Fatal(&ExpectedChangeError[E]{
			EventName: eventName,
			Matches:   matches,
			cfg:       cfg,
		})
		return
	}

	if matches == 0 || (cfg.exactly > 0 && matches != cfg.exactly) {
		t.Fatal(&ExpectedChangeError[E]{
			EventName:    eventName,
			Matches:      matches,
			cfg:          cfg,
			mismatchData: mismatchData,
		})
	}
}

// Change tests an Aggregate for a change. The Aggregate must not have an
// uncommitted change with the specified event name.
func NoChange[E comparable](t TestingT, a aggregate.Aggregate, eventName string, opts ...ChangeOption[E]) {
	var cfg changeConfig[E]
	for _, opt := range opts {
		opt(&cfg)
	}

	var matches int
	var zero E

	for _, change := range a.AggregateChanges() {
		if change.Name() != eventName {
			continue
		}

		if cfg.eventData != zero && !reflect.DeepEqual(cfg.eventData, change.Data()) {
			continue
		}

		matches++
	}

	if matches != 0 {
		t.Fatal(&UnexpectedChangeError{
			EventName: eventName,
		})
	}
}
