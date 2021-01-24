package eventbus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventbus/chanbus"
	bustest "github.com/modernice/goes/event/eventbus/test"
	"github.com/modernice/goes/event/eventstore/memstore"
	mock_event "github.com/modernice/goes/event/mocks"
	"github.com/modernice/goes/event/test"
)

func TestWithStore(t *testing.T) {
	bustest.EventBus(t, func(event.Encoder) event.Bus {
		return eventbus.WithStore(chanbus.New(), memstore.New())
	})

	store := memstore.New()
	bus := eventbus.WithStore(chanbus.New(), store)

	evt := event.New("foo", test.FooEventData{A: "foo"})
	if err := bus.Publish(context.Background(), evt); err != nil {
		t.Fatalf("publish %q event: %#v", "foo", err)
	}

	found, err := store.Find(context.Background(), evt.ID())
	if err != nil {
		t.Fatalf("expected store.Find to succeed; got %#v", err)
	}

	if !event.Equal(evt, found) {
		t.Errorf("found wrong event\n\nwant: %#v\n\ngot: %#v", evt, found)
	}
}

func TestWithStore_insertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evt := event.New("foo", test.FooEventData{A: "foo"})
	insertError := errors.New("insert error")

	mockBus := mock_event.NewMockBus(ctrl)
	mockStore := mock_event.NewMockStore(ctrl)
	mockStore.EXPECT().Insert(gomock.Any(), evt).Return(insertError)

	bus := eventbus.WithStore(mockBus, mockStore)
	if err := bus.Publish(context.Background(), evt); !errors.Is(err, insertError) {
		t.Fatalf("expected bus.Publish to fail with %#v; got %#v", insertError, err)
	}
}

func TestWithStore_storeFilter(t *testing.T) {
	filter := []func(evt event.Event) bool{
		func(evt event.Event) bool {
			return evt.AggregateName() != ""
		},
		func(evt event.Event) bool {
			return evt.AggregateID() != uuid.Nil
		},
	}

	tests := map[event.Event]bool{
		event.New("foo", test.FooEventData{A: "foo"}):                                        false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.Nil, 0)):   false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("", uuid.New(), 0)):    false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 0)): true,
	}

	for give, want := range tests {
		func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBus := mock_event.NewMockBus(ctrl)
			mockBus.EXPECT().Publish(gomock.Any(), give).Return(nil)

			mockStore := mock_event.NewMockStore(ctrl)
			if want {
				mockStore.EXPECT().Insert(gomock.Any(), give).Return(nil)
			}

			bus := eventbus.WithStore(mockBus, mockStore, eventbus.StoreFilter(filter...))

			if err := bus.Publish(context.Background(), give); err != nil {
				t.Fatalf("bus.Publish should not fail; got %#v", err)
			}
		}()
	}
}

func TestWithStore_requireAggregate(t *testing.T) {
	tests := map[event.Event]bool{
		event.New("foo", test.FooEventData{A: "foo"}):                                        false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.Nil, 0)):   false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("", uuid.New(), 0)):    false,
		event.New("foo", test.FooEventData{A: "foo"}, event.Aggregate("foo", uuid.New(), 0)): true,
	}

	for give, want := range tests {
		func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBus := mock_event.NewMockBus(ctrl)
			mockBus.EXPECT().Publish(gomock.Any(), give).Return(nil)

			mockStore := mock_event.NewMockStore(ctrl)
			if want {
				mockStore.EXPECT().Insert(gomock.Any(), give).Return(nil)
			}

			bus := eventbus.WithStore(mockBus, mockStore, eventbus.RequireAggregate())

			if err := bus.Publish(context.Background(), give); err != nil {
				t.Fatalf("bus.Publish should not fail; got %#v", err)
			}
		}()
	}
}
