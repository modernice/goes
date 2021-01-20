package consistency_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/consistency"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

func TestValidate_valid(t *testing.T) {
	aggregateID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 3)),
	}

	if err := consistency.Validate(b, events...); err != nil {
		t.Fatalf("expected validation to succeed; got %#v", err)
	}
}

func TestValidate_id(t *testing.T) {
	aggregateID := uuid.New()
	invalidID := uuid.New()
	b := aggregate.New("foo", aggregateID)
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", invalidID, 3)),
	}

	want := &consistency.Error{
		Kind:       consistency.ID,
		Aggregate:  b,
		Events:     events,
		EventIndex: 2,
	}

	if err := consistency.Validate(b, events...); !reflect.DeepEqual(err, want) {
		t.Fatalf("expected Validate to return %#v; got %#v", want, err)
	}
}

func TestValidate_name(t *testing.T) {
	aggregateID := uuid.New()
	aggregateName := "foo"
	invalidName := "bar"
	b := aggregate.New("foo", aggregateID)
	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate(invalidName, aggregateID, 3)),
	}

	want := &consistency.Error{
		Kind:       consistency.Name,
		Aggregate:  b,
		Events:     events,
		EventIndex: 2,
	}

	if err := consistency.Validate(b, events...); !reflect.DeepEqual(err, want) {
		t.Fatalf("expected Validate to return %#v; got %#v", want, err)
	}
}

func TestValidate_version(t *testing.T) {
	aggregateID := uuid.New()
	tests := []struct {
		name   string
		events []event.Event
		want   func(aggregate.Aggregate, []event.Event) *consistency.Error
	}{
		{
			name: "version too low #1",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 0)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *consistency.Error {
				return &consistency.Error{
					Kind:       consistency.Version,
					Aggregate:  a,
					Events:     events,
					EventIndex: 0,
				}
			},
		},
		{
			name: "version too low #2",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *consistency.Error {
				return &consistency.Error{
					Kind:       consistency.Version,
					Aggregate:  a,
					Events:     events,
					EventIndex: 2,
				}
			},
		},
		{
			name: "version skipped",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 3)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 4)),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *consistency.Error {
				return &consistency.Error{
					Kind:       consistency.Version,
					Aggregate:  a,
					Events:     events,
					EventIndex: 1,
				}
			},
		},
		{
			name: "duplicate version",
			events: []event.Event{
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 1)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
				event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", aggregateID, 2)),
			},
			want: func(a aggregate.Aggregate, events []event.Event) *consistency.Error {
				return &consistency.Error{
					Kind:       consistency.Version,
					Aggregate:  a,
					Events:     events,
					EventIndex: 2,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := aggregate.New("foo", aggregateID)
			err := consistency.Validate(a, tt.events...)
			want := tt.want(a, tt.events)

			cerr, ok := err.(*consistency.Error)
			if !ok {
				t.Fatalf("expected err to be a %T; got %T", &consistency.Error{}, err)
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

func TestError_Event(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	err := &consistency.Error{
		Kind:      consistency.UnknownKind,
		Aggregate: a,
		Events: []event.Event{
			event.New(
				"foo",
				test.FooEventData{A: "foo"},
				event.Aggregate(
					a.AggregateName(),
					a.AggregateID(),
					a.AggregateVersion(),
				),
			),
			event.New(
				"foo",
				test.FooEventData{A: "foo"},
				event.Aggregate(
					a.AggregateName(),
					a.AggregateID(),
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
	tests := map[*consistency.Error]string{
		{
			Kind: consistency.UnknownKind,
		}: fmt.Sprintf("consistency: invalid inconsistency kind=%d", consistency.UnknownKind),

		{
			Kind: consistency.Kind(9999),
		}: fmt.Sprintf("consistency: invalid inconsistency kind=%d", consistency.Kind(9999)),

		{
			Kind:      consistency.ID,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(name, invalidID, 1)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateID. want=%s got=%s", "foo", id, invalidID),

		{
			Kind:      consistency.Name,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(invalidName, id, 1)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateName. want=%s got=%s", "foo", name, invalidName),

		{
			Kind:      consistency.Version,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(name, id, 2)),
			},
			EventIndex: 0,
		}: fmt.Sprintf("consistency: %q event has invalid AggregateVersion. want=%d got=%d", "foo", 1, 2),

		{
			Kind:      consistency.Version,
			Aggregate: aggregate.New("foo", id),
			Events: []event.Event{
				event.New("foo", test.FooEventData{}, event.Aggregate(name, id, 1)),
				event.New("foo", test.FooEventData{}, event.Aggregate(name, id, 3)),
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
