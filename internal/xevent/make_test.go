package xevent_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/internal/xevent"
)

func TestMake(t *testing.T) {
	data := test.FooEventData{A: "foo"}
	events := xevent.Make("foo", data, 10)
	if len(events) != 10 {
		t.Errorf("xevent.Make(%d) should return %d events; got %d", 10, 10, len(events))
	}

	for _, evt := range events {
		want := event.New[any]("foo", data, event.ID[any](evt.ID()), event.Time[any](evt.Time()))
		if !event.Equal(evt, want.Event()) {
			t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
		}
	}
}

func TestMake_forAggregate(t *testing.T) {
	data := test.FooEventData{A: "foo"}
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", data, 10, xevent.ForAggregate(a))

	for i, evt := range events {
		want := event.New[any](
			"foo",
			data,
			event.ID[any](evt.ID()),
			event.Time[any](evt.Time()),
			event.Aggregate[any](a.AggregateID(), a.AggregateName(), i+1),
		)
		if !event.Equal(evt, want.Event()) {
			t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
		}
	}
}

func TestMake_forAggregate_many(t *testing.T) {
	data := test.FooEventData{A: "foo"}
	as := []aggregate.Aggregate{
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
		aggregate.New("foo", uuid.New()),
	}
	events := xevent.Make("foo", data, 10, xevent.ForAggregate(as[0]), xevent.ForAggregate(as[1:]...))

	if len(events) != 30 {
		t.Errorf("Make should return %d (%d * %d) events; got %d", 30, 3, 10, len(events))
	}

	for _, a := range as {
		id, name, _ := a.Aggregate()
		for i, evt := range eventsFor(id, events...) {
			want := event.New[any](
				"foo",
				data,
				event.ID[any](evt.ID()),
				event.Time[any](evt.Time()),
				event.Aggregate[any](id, name, i+1),
			)
			if !event.Equal(evt, want.Event()) {
				t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
			}
		}
	}
}

func TestMake_skipVersion(t *testing.T) {
	data := test.FooEventData{A: "foo"}
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", data, 10, xevent.ForAggregate(a), xevent.SkipVersion(2, 6))

	var skipped int
	for i, evt := range events {
		v := i + 1
		switch v {
		case 2, 6:
			skipped++
		}
		v += skipped
		want := event.New[any](
			"foo",
			data,
			event.ID[any](evt.ID()),
			event.Time[any](evt.Time()),
			event.Aggregate[any](a.AggregateID(), a.AggregateName(), v),
		)
		if !event.Equal(evt, want.Event()) {
			t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
		}
	}
}

func eventsFor(id uuid.UUID, events ...event.Event) []event.Event {
	result := make([]event.Event, 0, len(events))
	for _, evt := range events {
		if pick.AggregateID(evt) == id {
			result = append(result, evt)
		}
	}
	return result
}
