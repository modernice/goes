package aggregate_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
	"github.com/modernice/goes/internal/xtime"
)

func TestValidate_valid(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	now := xtime.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Nanosecond))),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3), event.Time(now.Add(time.Millisecond))),
	}

	if err := aggregate.ValidateConsistency(b, events...); err != nil {
		t.Fatalf("expected validation to succeed; got %#v", err)
	}
}

func TestValidate_id(t *testing.T) {
	aggregateID := uuid.New()
	invalidID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	now := xtime.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Nanosecond))),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(invalidID, "foo", 3), event.Time(now.Add(time.Millisecond))),
	}

	want := &aggregate.ConsistencyError{
		Kind:       aggregate.InconsistentID,
		Aggregate:  b,
		Events:     events,
		EventIndex: 2,
	}

	if err := aggregate.ValidateConsistency(b, events...); !reflect.DeepEqual(err, want) {
		t.Fatalf("expected Validate to return %#v; got %#v", want, err)
	}
}

func TestValidate_name(t *testing.T) {
	aggregateID := uuid.New()
	aggregateName := "foo"
	invalidName := "bar"
	b := aggregate.New("foo", aggregateID)
	now := xtime.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, aggregateName, 1), event.Time(now)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, aggregateName, 2), event.Time(now.Add(time.Nanosecond))),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, invalidName, 3), event.Time(now.Add(time.Millisecond))),
	}

	want := &aggregate.ConsistencyError{
		Kind:       aggregate.InconsistentName,
		Aggregate:  b,
		Events:     events,
		EventIndex: 2,
	}

	if err := aggregate.ValidateConsistency(b, events...); !reflect.DeepEqual(err, want) {
		t.Fatalf("expected Validate to return %#v; got %#v", want, err)
	}
}

func TestValidate_version(t *testing.T) {
	aggregateID := uuid.New()
	changedAggregate := aggregate.New("foo", aggregateID)
	changes := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(changedAggregate))
	changedAggregate.TrackChange(changes...)
	now := xtime.Now()

	tests := []struct {
		name      string
		aggregate aggregate.Aggregate
		events    []event.Event
		want      func(aggregate.Aggregate, []event.Event) *aggregate.ConsistencyError
	}{
		{
			name: "version too low #1",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 0), event.Time(now)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now.Add(time.Nanosecond))),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Millisecond))),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *aggregate.ConsistencyError {
				return &aggregate.ConsistencyError{
					Kind:       aggregate.InconsistentVersion,
					Aggregate:  a,
					Events:     events,
					EventIndex: 0,
				}
			},
		},
		{
			name: "version too low #2",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Nanosecond))),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Millisecond))),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *aggregate.ConsistencyError {
				return &aggregate.ConsistencyError{
					Kind:       aggregate.InconsistentVersion,
					Aggregate:  a,
					Events:     events,
					EventIndex: 2,
				}
			},
		},
		{
			name:      "version too low #3 (with changes)",
			aggregate: changedAggregate,
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Nanosecond))),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3), event.Time(now.Add(time.Millisecond))),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *aggregate.ConsistencyError {
				return &aggregate.ConsistencyError{
					Kind:       aggregate.InconsistentVersion,
					Aggregate:  a,
					Events:     events,
					EventIndex: 0,
				}
			},
		},
		{
			name: "version skipped",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 3), event.Time(now.Add(time.Nanosecond))),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 4), event.Time(now.Add(time.Millisecond))),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *aggregate.ConsistencyError {
				return &aggregate.ConsistencyError{
					Kind:       aggregate.InconsistentVersion,
					Aggregate:  a,
					Events:     events,
					EventIndex: 1,
				}
			},
		},
		{
			name: "duplicate version",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1), event.Time(now)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Nanosecond))),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2), event.Time(now.Add(time.Millisecond))),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *aggregate.ConsistencyError {
				return &aggregate.ConsistencyError{
					Kind:       aggregate.InconsistentVersion,
					Aggregate:  a,
					Events:     events,
					EventIndex: 2,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.aggregate
			if a == nil {
				a = aggregate.New("foo", aggregateID)
			}
			err := aggregate.ValidateConsistency(a, tt.events...)
			want := tt.want(a, tt.events)

			cerr, ok := err.(*aggregate.ConsistencyError)
			if !ok {
				t.Fatalf("expected err to be a %T; got %T", &aggregate.ConsistencyError{}, err)
			}

			if cerr.Aggregate != want.Aggregate ||
				!test.EqualEvents(tt.events, cerr.Events) ||
				cerr.Kind != want.Kind ||
				cerr.EventIndex != want.EventIndex {
				t.Fatalf("Validate returnded wrong events\n\nwant: %#v\n\ngot: %#v\n\n", want, cerr)
			}
		})
	}
}

func TestValidate_time(t *testing.T) {
	id := uuid.New()
	now := xtime.Now()
	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate(id, "foo", 1), event.Time(now)),
		event.New("bar", test.BarEventData{}, event.Aggregate(id, "foo", 2), event.Time(now.Add(time.Nanosecond))),
		event.New("baz", test.BazEventData{}, event.Aggregate(id, "foo", 3), event.Time(now.Add(time.Nanosecond))),
		event.New("foo", test.FooEventData{}, event.Aggregate(id, "foo", 4), event.Time(now.Add(2*time.Nanosecond))),
		event.New("bar", test.BarEventData{}, event.Aggregate(id, "foo", 5), event.Time(now.Add(time.Second))),
		event.New("baz", test.BazEventData{}, event.Aggregate(id, "foo", 6), event.Time(now.Add(time.Minute))),
	}

	a := aggregate.New("foo", id)

	err := aggregate.ValidateConsistency(a, events...)

	var consistencyErr *aggregate.ConsistencyError
	if !errors.As(err, &consistencyErr) {
		t.Fatalf("Validate should return a %T; got %T", consistencyErr, err)
	}

	if consistencyErr.Event() != events[2] {
		t.Fatalf("Event should return %v; got %v", events[2], consistencyErr.Event())
	}

	if consistencyErr.Kind != aggregate.InconsistentTime {
		t.Fatalf("Kind should be %v; got %v", aggregate.InconsistentTime, consistencyErr.Kind)
	}
}

func TestError_Event(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	err := &aggregate.ConsistencyError{
		Kind:      aggregate.ConsistencyKind(0),
		Aggregate: a,
		Events: []event.Event{
			event.New(
				"foo",
				test.FooEventData{A: "foo"},
				event.Aggregate(
					a.AggregateID(),
					a.AggregateName(),
					a.AggregateVersion(),
				),
			),
			event.New(
				"foo",
				test.FooEventData{A: "foo"},
				event.Aggregate(
					a.AggregateID(),
					a.AggregateName(),
					a.AggregateVersion(),
				),
			),
		},
		EventIndex: 1,
	}

	if evt := err.Event(); !event.Equal(evt, err.Events[1]) {
		t.Fatalf("expected err.Event to return %#v; got %#v", err.Events[1], evt)
	}
}

func TestError_Error(t *testing.T) {
	id := uuid.New()
	name := "foo"
	invalidID := uuid.New()
	invalidName := "bar"
	tests := map[*aggregate.ConsistencyError]string{
		{
			Kind: aggregate.ConsistencyKind(0),
		}: fmt.Sprintf("consistency: invalid inconsistency kind=%d", aggregate.ConsistencyKind(0)),

		{
			Kind: aggregate.ConsistencyKind(9999),
		}: fmt.Sprintf("consistency: invalid inconsistency kind=%d", aggregate.ConsistencyKind(9999)),

		{
			Kind:      aggregate.InconsistentID,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(invalidID, name, 1)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateID. want=%s got=%s", "foo", id, invalidID),

		{
			Kind:      aggregate.InconsistentName,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(id, invalidName, 1)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateName. want=%s got=%s", "foo", name, invalidName),

		{
			Kind:      aggregate.InconsistentVersion,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(id, name, 2)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateVersion. want=%d got=%d", "foo", 1, 2),

		{
			Kind:      aggregate.InconsistentVersion,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(id, name, 1)),
				event.New("foo", test.FooEventData{}, event.Aggregate(id, name, 3)),
			},
			EventIndex: 1,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateVersion. want=%d got=%d", "foo", 2, 3),
	}

	for give, want := range tests {
		if msg := give.Error(); msg != want {
			t.Errorf("expected error message %q; got %q", want, msg)
		}
	}
}
