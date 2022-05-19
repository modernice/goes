//go:build mongo

package mongo_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/backend/mongo/mongotest"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	etest "github.com/modernice/goes/event/test"
	"go.mongodb.org/mongo-driver/bson"
)

func TestEventStore(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		eventstoretest.Run(t, "mongostore", func(enc codec.Encoding) event.Store {
			return mongotest.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")))
		})
	})

	t.Run("ReplicaSet", func(t *testing.T) {
		eventstoretest.Run(t, "mongostore", func(enc codec.Encoding) event.Store {
			return mongotest.NewEventStore(
				enc,
				mongo.URL(os.Getenv("MONGOREPLSTORE_URL")),
				mongo.Transactions(true),
			)
		})
	})
}

func TestEventStore_Insert_versionError(t *testing.T) {
	enc := etest.NewEncoder()
	s := mongo.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")))

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
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), a.AggregateVersion()+5)),
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), a.AggregateVersion()+6)),
		event.New[any]("foo", etest.FooEventData{}, event.Aggregate(a.AggregateID(), a.AggregateName(), a.AggregateVersion()+7)),
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
