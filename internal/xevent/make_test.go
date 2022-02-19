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
	events := xevent.Make(uuid.New, "foo", data, 10)
	if len(events) != 10 {
		t.Errorf("xevent.Make(%d) should return %d events; got %d", 10, 10, len(events))
	}

	for _, evt := range events {
		want := event.New[any](evt.ID(), "foo", data, event.ID(evt.ID()), event.Time(evt.Time()))
		if !event.Equal(evt, want.Event()) {
			t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
		}
	}
}

func TestMake_forAggregate(t *testing.T) {
	data := test.FooEventData{A: "foo"}
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make(uuid.New, "foo", data, 10, xevent.ForAggregate[uuid.UUID](a))

	for i, evt := range events {
		want := event.New[any](
			evt.ID(),
			"foo",
			data,
			event.Time(evt.Time()),
			event.Aggregate(a.AggregateID(), a.AggregateName(), i+1),
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
	events := xevent.Make(uuid.New, "foo", data, 10, xevent.ForAggregate(as[0]), xevent.ForAggregate(as[1:]...))

	if len(events) != 30 {
		t.Errorf("Make should return %d (%d * %d) events; got %d", 30, 3, 10, len(events))
	}

	for _, a := range as {
		id, name, _ := a.Aggregate()
		for i, evt := range eventsFor(id, events...) {
			want := event.New[any](
				evt.ID(),
				"foo",
				data,
				event.ID(evt.ID()),
				event.Time(evt.Time()),
				event.Aggregate(id, name, i+1),
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
	events := xevent.Make(uuid.New, "foo", data, 10, xevent.ForAggregate[uuid.UUID](a), xevent.SkipVersion[uuid.UUID](2, 6))

	var skipped int
	for i, evt := range events {
		v := i + 1
		switch v {
		case 2, 6:
			skipped++
		}
		v += skipped
		want := event.New[any](
			evt.ID(),
			"foo",
			data,
			event.ID(evt.ID()),
			event.Time(evt.Time()),
			event.Aggregate(a.AggregateID(), a.AggregateName(), v),
		)
		if !event.Equal(evt, want.Event()) {
			t.Errorf("made wrong event\n\nwant: %#v\n\ngot: %#v\n\n", want, evt)
		}
	}
}

func eventsFor(id uuid.UUID, events ...event.Of[any, uuid.UUID]) []event.Of[any, uuid.UUID] {
	result := make([]event.Of[any, uuid.UUID], 0, len(events))
	for _, evt := range events {
		if pick.AggregateID[uuid.UUID](evt) == id {
			result = append(result, evt)
		}
	}
	return result
}
