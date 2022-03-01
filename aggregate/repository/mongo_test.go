//go:build mongo

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/backend/mongo"
	"github.com/modernice/goes/event"
	etest "github.com/modernice/goes/event/test"
)

func TestRetryUse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enc := etest.NewEncoder()
	estore := mongo.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")))

	r := repository.New(estore, repository.RetryUse(3, 50*time.Millisecond, mongo.IsVersionError))

	foo := test.NewFoo(uuid.New())

	events := []event.Event{
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
	}

	aggregate.ApplyHistory(foo, events)

	r.Save(ctx, foo)

	foo = test.NewFoo(foo.AggregateID())

	var tries int
	start := time.Now()
	if err := r.Use(ctx, foo, func() error {
		tries++
		// apply the last event again. this should fail with a *mongo.VersionError
		foo.TrackChange(events[len(events)-1])
		return nil
	}); !mongo.IsVersionError(err) {
		t.Fatalf("Use() should fail with a %T; got %q", &mongo.VersionError{}, err)
	}

	if tries != 3 {
		t.Fatalf("Use() should have tried 3 times; tried %d times", tries)
	}

	dur := time.Since(start)
	if dur.Milliseconds() < 150 {
		t.Fatalf("Use() should have taken at least %v; took %s", 150*time.Millisecond, dur)
	}

	if dur.Milliseconds() > 250 {
		t.Fatalf("Use() should have taken ~%v; took %v", 150*time.Millisecond, dur)
	}

}
