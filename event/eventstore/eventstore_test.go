package eventstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	mock_event "github.com/modernice/goes/event/mocks"
	"github.com/modernice/goes/event/test"
)

func TestWithBus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	base := mock_event.NewMockStore(ctrl)
	bus := mock_event.NewMockBus(ctrl)
	store := eventstore.WithBus(base, bus)

	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}),
		event.New("bar", test.BarEventData{A: "bar"}),
		event.New("baz", test.BazEventData{A: "baz"}),
	}

	base.EXPECT().Insert(gomock.Any(), events).Return(nil)
	bus.EXPECT().Publish(gomock.Any(), events).Return(nil)

	err := store.Insert(context.Background(), events...)
	if err != nil {
		t.Fatalf("insert should not fail; got %#v", err)
	}
}

func TestWithBus_insertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	base := mock_event.NewMockStore(ctrl)
	bus := mock_event.NewMockBus(ctrl)
	store := eventstore.WithBus(base, bus)

	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}),
		event.New("bar", test.BarEventData{A: "bar"}),
		event.New("baz", test.BazEventData{A: "baz"}),
	}

	insertError := errors.New("insert error")
	base.EXPECT().Insert(gomock.Any(), events).Return(insertError)

	err := store.Insert(context.Background(), events...)
	if !errors.Is(err, insertError) {
		t.Fatalf("expected insert to fail with %#v; got %#v", insertError, err)
	}
}

func TestWithBus_publishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	base := mock_event.NewMockStore(ctrl)
	bus := mock_event.NewMockBus(ctrl)
	store := eventstore.WithBus(base, bus)

	events := []event.Event{
		event.New("foo", test.FooEventData{A: "foo"}),
		event.New("bar", test.BarEventData{A: "bar"}),
		event.New("baz", test.BazEventData{A: "baz"}),
	}

	publishError := errors.New("insert error")
	base.EXPECT().Insert(gomock.Any(), events).Return(nil)
	bus.EXPECT().Publish(gomock.Any(), events).Return(publishError)

	err := store.Insert(context.Background(), events...)
	if !errors.Is(err, publishError) {
		t.Fatalf("expected insert to fail with %#v; got %#v", publishError, err)
	}
}
