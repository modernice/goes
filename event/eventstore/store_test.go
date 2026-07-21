package eventstore_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/test"
)

var _ event.Store = eventstore.New()

func TestMemstore(t *testing.T) {
	eventstoretest.Run(t, "memstore", func(codec.Encoding) event.Store {
		return eventstore.New()
	})
}

func TestMemstore_VersionConflict(t *testing.T) {
	store := eventstore.New()
	aggregateID := uuid.New()

	first := event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1))
	if err := store.Insert(context.Background(), first); err != nil {
		t.Fatalf("insert first event: %v", err)
	}

	// Inserting an event for an aggregate version that already exists fails.
	conflicting := event.New[any]("bar", test.BarEventData{A: "bar"}, event.Aggregate(aggregateID, "foo", 1))
	if err := store.Insert(context.Background(), conflicting); err == nil {
		t.Fatal("expected insert into an existing aggregate version to fail")
	}
	if _, err := store.Find(context.Background(), conflicting.ID()); err == nil {
		t.Fatal("conflicting event should not have been inserted")
	}

	// A failed insert must not apply any event of the batch.
	batch := []event.Event{
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 2)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1)),
	}
	if err := store.Insert(context.Background(), batch...); err == nil {
		t.Fatal("expected batch with conflicting aggregate version to fail")
	}
	if _, err := store.Find(context.Background(), batch[0].ID()); err == nil {
		t.Fatal("failed batch should not have inserted any event")
	}

	// Events without a full aggregate reference are exempt, mirroring the
	// partial unique index of the MongoDB event store.
	loose := []event.Event{
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 0)),
		event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 0)),
	}
	if err := store.Insert(context.Background(), loose...); err != nil {
		t.Fatalf("insert events without full aggregate reference: %v", err)
	}

	// Deleting an event frees its aggregate version.
	if err := store.Delete(context.Background(), first); err != nil {
		t.Fatalf("delete first event: %v", err)
	}
	replacement := event.New[any]("foo", test.FooEventData{A: "foo"}, event.Aggregate(aggregateID, "foo", 1))
	if err := store.Insert(context.Background(), replacement); err != nil {
		t.Fatalf("insert into freed aggregate version: %v", err)
	}
}
