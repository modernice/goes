//go:build mongo

package repository_test

import (
	"context"
	"log"
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

	// First try has no delay, the remainining 3 tries are delayed by 50ms
	r := repository.New(estore, repository.RetryUse(repository.RetryEvery(50*time.Millisecond, 4), mongo.IsConsistencyError))

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
	}); !mongo.IsConsistencyError(err) {
		t.Fatalf("Use() should fail with a %T; got %q", &mongo.VersionError{}, err)
	}

	if tries != 4 {
		t.Fatalf("Use() should have tried 4 times; tried %d times", tries)
	}

	dur := time.Since(start)
	if dur.Milliseconds() < 150 {
		t.Fatalf("Use() should have taken at least %v; took %s", 150*time.Millisecond, dur)
	}

	if dur.Milliseconds() > 250 {
		t.Fatalf("Use() should have taken ~%v; took %v", 150*time.Millisecond, dur)
	}
}

func TestRetryProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enc := etest.NewEncoder()
	estore := mongo.NewEventStore(enc, mongo.URL(os.Getenv("MONGOSTORE_URL")))

	r := repository.New(estore)

	foo := newRetryer()

	events := []event.Event{
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
		aggregate.NextEvent(foo, "foo", etest.FooEventData{}).Any(),
	}

	aggregate.ApplyHistory(foo, events)

	r.Save(ctx, foo)

	start := time.Now()
	var tries int
	if err := r.Use(ctx, foo, func() error {
		log.Println("TRY")
		tries++
		// apply the last event again. this should fail with a *mongo.VersionError
		foo.TrackChange(events[len(events)-1])
		return nil
	}); !mongo.IsConsistencyError(err) {
		t.Fatalf("Use() should fail with a consistency error; got %q", err)
	}

	if dur := time.Since(start); dur.Milliseconds() < 150 || dur.Milliseconds() > 250 {
		t.Fatalf("Use() should have taken ~%v; took %v", 150*time.Millisecond, dur)
	}

	if tries != 4 {
		t.Fatalf("Use() should have tried 4 times; tried %d times", tries)
	}
}

type retryer struct{ *aggregate.Base }

func newRetryer() *retryer {
	return &retryer{
		Base: aggregate.New("retryer", uuid.New()),
	}
}

func (r *retryer) RetryUse() (repository.RetryTrigger, repository.IsRetryable) {
	return repository.RetryEvery(50*time.Millisecond, 4), mongo.IsConsistencyError
}
