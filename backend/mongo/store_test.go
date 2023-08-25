//go:build mongo

package mongo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodb "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/backend/mongo/mongotest"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	etest "github.com/modernice/goes/event/test"
)

func TestEventStore(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		eventstoretest.Run(t, "mongostore", func(enc codec.Encoding) event.Store {
			return mongotest.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")), mongo.Database(nextEventDatabase()))
		})
	})

	t.Run("ReplicaSet", func(t *testing.T) {
		eventstoretest.Run(t, "mongostore", func(enc codec.Encoding) event.Store {
			return mongotest.NewEventStore(
				enc,
				mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
				mongo.Transactions(true),
				mongo.Database(nextEventDatabase()),
			)
		})
	})
}

func TestEventStore_Insert_versionError(t *testing.T) {
	enc := etest.NewEncoder()
	s := mongo.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")), mongo.Database(nextEventDatabase()))

	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	a := aggregate.New("foo", uuid.New())

	states := s.StateCollection()
	if _, err := states.InsertOne(context.Background(), bson.M{
		"aggregateName": "foo",
		"aggregateId":   a.AggregateID(),
		"version":       5,
	}); err != nil {
		t.Fatalf("failed to insert state: %v", err)
	}

	events := []event.Event{
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 5)),
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 6)),
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 7)),
	}

	err := s.Insert(context.Background(), events...)

	var versionError mongo.VersionError
	if !errors.As(err, &versionError) {
		t.Fatalf("Insert should fail a %T error; got %T", versionError, err)
	}

	if versionError.AggregateName != "foo" {
		t.Errorf("VersionError should have AggregateName %q; got %q", "foo", versionError.AggregateName)
	}

	if versionError.AggregateID != a.AggregateID() {
		t.Errorf("VersionError should have AggregateID %s; got %s", a.AggregateID(), versionError.AggregateID)
	}

	if versionError.CurrentVersion != 5 {
		t.Errorf("VersionError should have CurrentVersion %d; got %d", 5, a.AggregateVersion())
	}

	if versionError.Event != events[0] {
		t.Errorf("VersionError should have Event %v; got %v", events[0], versionError.Event)
	}
}

// TestEventStore_Insert_preAndPostHooks tests the following scenario
// Given: [0: "insert:pre", 1: "insert:pre", 2: "insert:post", 3: "insert:post"] hooks, then
//
// for hook 0: InsertedEvents() returns empty slice
// for hook 1: InsertedEvents() returns events that were inserted by hook 0
// for hook 2: InsertedEvents() returns events that were inserted by hook 0 and 1, and the "main" events
// for hook 3: InsertedEvents() returns events that were inserted by hook 0 and 1, the "main" events, and the events inserted by hook 2
func TestEventStore_Insert_withPreAndPostHooks(t *testing.T) {
	enc := etest.NewEncoder()

	a := aggregate.New("foo", uuid.New())
	expectedEvent0 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 1))
	expectedEvent1 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 2))
	expectedEvent2 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 3))
	expectedEvent3 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 4))
	expectedEvent4 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 5))
	allEvents := []event.Event{expectedEvent0, expectedEvent1, expectedEvent2, expectedEvent3, expectedEvent4}

	hookObserver0 := aHookObserver(func(ctx mongo.TransactionContext) error { return ctx.EventStore().Insert(ctx, expectedEvent0) })
	hookObserver1 := aHookObserver(func(ctx mongo.TransactionContext) error { return ctx.EventStore().Insert(ctx, expectedEvent1) })
	hookObserver2 := aHookObserver(func(ctx mongo.TransactionContext) error { return ctx.EventStore().Insert(ctx, expectedEvent3) })
	hookObserver3 := aHookObserver(func(ctx mongo.TransactionContext) error { return ctx.EventStore().Insert(ctx, expectedEvent4) })

	s := mongotest.NewEventStore(
		enc,
		mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
		mongo.Database(nextEventDatabase()),
		mongo.WithTransactionHook(mongo.PreInsert, hookObserver0.hook),
		mongo.WithTransactionHook(mongo.PreInsert, hookObserver1.hook),
		mongo.WithTransactionHook(mongo.PostInsert, hookObserver2.hook),
		mongo.WithTransactionHook(mongo.PostInsert, hookObserver3.hook),
	)

	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	err := s.Insert(context.Background(), expectedEvent2)
	if err != nil {
		t.Errorf("failed to insert event: %s", err)
	}

	type testData struct {
		name     string
		observer *hookObserver
		events   []event.Event
	}

	tests := []testData{
		{
			"observer0",
			hookObserver0,
			nil,
		},
		{
			"observer1",
			hookObserver1,
			[]event.Event{expectedEvent0},
		},
		{
			"observer2",
			hookObserver2,
			[]event.Event{expectedEvent0, expectedEvent1, expectedEvent2},
		},
		{
			"observer3",
			hookObserver3,
			[]event.Event{expectedEvent0, expectedEvent1, expectedEvent2, expectedEvent3},
		},
	}

	for _, tt := range tests {
		if tt.observer.called != 1 {
			t.Errorf("hook %q was not called once", tt.name)
		}
		if !cmp.Equal(tt.observer.insertedEvents, tt.events) {
			t.Errorf("hook inserted events dont match expected\nhook: %#v\n\ninserted: %#v\n\ndiff: %s", tt.observer.insertedEvents, tt.events, cmp.Diff(
				tt.observer.insertedEvents, tt.events,
			))
		}
	}
	for _, expected := range allEvents {
		found, err := s.Find(context.Background(), expected.ID())
		if err != nil {
			t.Fatalf("expected store.Find not to return error; got %#v", err)
		}

		if !event.Equal(found, expected) {
			t.Errorf("found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v\n\ndiff: %s", expected, found, cmp.Diff(
				expected, found, cmp.AllowUnexported(expected),
			))
		}
	}
}

func TestEventStore_Insert_preHookInsertingIntoCollection(t *testing.T) {
	enc := etest.NewEncoder()

	dbName := nextEventDatabase()
	client, testCollection, err := aCollection(dbName)
	if err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}
	defer client.Disconnect(context.TODO())

	a := aggregate.New("foo", uuid.New())
	expectedEvent0 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 1))
	expectedEvent1 := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 2))
	allEvents := []event.Event{expectedEvent0, expectedEvent1}
	expectedEntity := testEntity{Name: "Test"}

	hookObserver0 := aHookObserver(func(ctx mongo.TransactionContext) error {
		result, err := testCollection.InsertOne(ctx, expectedEntity)
		if err != nil {
			return err
		}
		id, _ := result.InsertedID.(primitive.ObjectID)
		expectedEntity.ID = id
		return nil
	})
	hookObserver1 := aHookObserver(func(ctx mongo.TransactionContext) error { return ctx.EventStore().Insert(ctx, expectedEvent1) })

	s := mongotest.NewEventStore(
		enc,
		mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
		mongo.Client(client),
		mongo.Database(dbName),
		mongo.WithTransactionHook(mongo.PreInsert, hookObserver0.hook),
		mongo.WithTransactionHook(mongo.PostInsert, hookObserver1.hook),
	)

	if _, err = s.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	err = s.Insert(context.Background(), expectedEvent0)
	if err != nil {
		t.Errorf("error inserting %s", err)
	}

	if hookObserver0.called != 1 {
		t.Errorf("hook 1 was not called once")
	}
	if hookObserver1.called != 1 {
		t.Errorf("hook 2 was not called once")
	}
	for _, expected := range allEvents {
		found, err := s.Find(context.Background(), expected.ID())
		if err != nil {
			t.Fatalf("expected store.Find not to return error; got %#v", err)
		}
		if !event.Equal(found, expected) {
			t.Errorf("found event doesn't match inserted event\ninserted: %#v\n\nfound: %#v\n\ndiff: %s", expected, found, cmp.Diff(
				expected, found, cmp.AllowUnexported(expected),
			))
		}
	}

	var actual testEntity
	err = testCollection.FindOne(context.Background(), bson.M{"_id": expectedEntity.ID}).Decode(&actual)
	if err != nil {
		t.Errorf("expected to find entity, got err: %s", err)
	}
	if actual != expectedEntity {
		t.Errorf("actual entity doesnt match the expected entity\nactual: %#v\n\nexpected: %#v\n\ndiff: %s", actual, expectedEntity, cmp.Diff(
			actual, expectedEntity, cmp.AllowUnexported(expectedEntity),
		))
	}

	if !cmp.Equal(hookObserver1.insertedEvents, []event.Event{expectedEvent0}) {
		t.Errorf("hook inserted events dont match expected\nhook: %#v\n\ninserted: %#v\n\ndiff: %s", hookObserver1.insertedEvents, []event.Event{expectedEvent0}, cmp.Diff(
			hookObserver1.insertedEvents, []event.Event{expectedEvent0},
		))
	}
}

func TestEventStore_Insert_with3HooksLastFailing(t *testing.T) {
	enc := etest.NewEncoder()

	dbName := nextEventDatabase()
	client, testCollection, err := aCollection(dbName)
	if err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}
	defer client.Disconnect(context.TODO())

	a := aggregate.New("foo", uuid.New())
	mainEvent := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 2))
	preEvent := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 1))
	postEvent := event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), 3))
	allEvents := []event.Event{preEvent, mainEvent, postEvent}
	expectedEntity := testEntity{Name: "Test"}
	expectedErr := errors.New("some error in the handler")

	preObserver := aHookObserver(func(ctx mongo.TransactionContext) error {
		return ctx.EventStore().Insert(ctx, preEvent)
	})
	postObserver := aHookObserver(func(ctx mongo.TransactionContext) error {
		return ctx.EventStore().Insert(ctx, postEvent)
	})
	failingObserver := aHookObserver(func(ctx mongo.TransactionContext) error {
		result, err := testCollection.InsertOne(ctx, expectedEntity)
		if err != nil {
			return err
		}
		id, _ := result.InsertedID.(primitive.ObjectID)
		expectedEntity.ID = id
		return expectedErr
	})

	s := mongotest.NewEventStore(
		enc,
		mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
		mongo.Client(client),
		mongo.Database(dbName),
		mongo.WithTransactionHook(mongo.PreInsert, preObserver.hook),
		mongo.WithTransactionHook(mongo.PostInsert, postObserver.hook),
		mongo.WithTransactionHook(mongo.PostInsert, failingObserver.hook),
	)

	if _, err = s.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	err = s.Insert(context.Background(), mainEvent)
	if !errors.Is(err, expectedErr) {
		t.Errorf("not expected error: %s", err)
	}

	if preObserver.called != 1 {
		t.Errorf("pre-insert hook was not called once")
	}
	if postObserver.called != 1 {
		t.Errorf("post-insert hook was not called once")
	}

	for _, expected := range allEvents {
		found, err := s.Find(context.Background(), expected.ID())
		if err == nil {
			t.Errorf("expected store.Find() to return error after a failed transaction")
		}
		if found != nil {
			t.Errorf("found event, expected nil: %v", found)
		}
	}

	var actual *testEntity
	result := testCollection.FindOne(context.Background(), bson.M{"_id": expectedEntity.ID})

	if result.Err() == nil {
		t.Errorf("expected error to find entity %q, got nil", expectedEntity.ID)
	}
	if !errors.Is(result.Err(), mongodb.ErrNoDocuments) {
		t.Errorf("expected: '%s', got err: '%s'", mongodb.ErrNoDocuments, result.Err())
	}

	if err := result.Decode(&actual); err == nil {
		t.Errorf("expected error to decode entity %q, got nil", expectedEntity.ID)
	}

	if actual != nil {
		t.Errorf("expected nil document, got: %v", actual)
	}
}

func TestEventStore_WithTxHook_failsWithoutTransactionsEnabled(t *testing.T) {
	enc := etest.NewEncoder()
	hook := func(ctx mongo.TransactionContext) error {
		return nil
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("the code did not panic")
		}
	}()

	_ = mongotest.NewEventStore(
		enc,
		mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
		mongo.Database(nextEventDatabase()),
		mongo.WithTransactionHook(mongo.PostInsert, hook),
		mongo.Transactions(false),
	)
}

var evtDBID uint64

func nextEventDatabase() string {
	id := atomic.AddUint64(&evtDBID, 1)
	return fmt.Sprintf("events_%d", id)
}

type hookObserver struct {
	hook           func(hook mongo.TransactionContext) error
	called         int
	insertedEvents []event.Event
}

func aHookObserver(fns ...func(mongo.TransactionContext) error) *hookObserver {
	obs := &hookObserver{}
	hook := func(ctx mongo.TransactionContext) error {
		obs.called++
		obs.insertedEvents = ctx.InsertedEvents()
		for _, fn := range fns {
			err := fn(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	obs.hook = hook
	return obs
}

type testEntity struct {
	ID   primitive.ObjectID `bson:"_id,omitempty"`
	Name string             `bson:"name,omitempty"`
}

func aCollection(dbName string) (*mongodb.Client, *mongodb.Collection, error) {
	client, err := mongodb.Connect(context.TODO(),
		options.Client().ApplyURI(os.Getenv("MONGOREPLSTORE_URL")))
	if err != nil {
		return nil, nil, err
	}

	colname := mongotest.UniqueName("hooks_")
	db := client.Database(dbName)
	db.RunCommand(context.Background(), bson.D{{Key: "create", Value: colname}})
	testCollection := db.Collection(colname)

	return client, testCollection, nil
}
