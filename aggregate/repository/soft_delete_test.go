package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event/eventstore"
	etest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
)

func TestRepository_Fetch_SoftDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	estore := eventstore.New()
	r := repository.New(estore)

	foo := test.NewFoo(uuid.New())

	aggregate.Next(foo, "foo", etest.FooEventData{}).Any()
	aggregate.Next(foo, "soft_deleted", softDeletedEvent{}).Any()

	r.Save(ctx, foo)

	foo = test.NewFoo(foo.AggregateID())

	if err := r.Fetch(ctx, foo); !errors.Is(err, repository.ErrDeleted) {
		t.Fatalf("Fetch() should fail with %q; got %q", repository.ErrDeleted, err)
	}
}

func TestRepository_Fetch_SoftRestore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	estore := eventstore.New()
	r := repository.New(estore)

	foo := test.NewFoo(uuid.New())

	aggregate.Next(foo, "foo", etest.FooEventData{}).Any()
	aggregate.Next(foo, "soft_deleted", softDeletedEvent{}).Any()
	aggregate.Next(foo, "foo", etest.FooEventData{}).Any()
	aggregate.Next(foo, "soft_restored", softRestoredEvent{}).Any()

	r.Save(ctx, foo)

	foo = test.NewFoo(foo.AggregateID())

	if err := r.Fetch(ctx, foo); err != nil {
		t.Fatalf("Fetch() failed with %q", err)
	}
}

func TestRepository_Query_SoftDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	estore := eventstore.New()
	r := repository.New(estore)

	foo := test.NewFoo(uuid.New())
	bar := test.NewFoo(uuid.New())

	aggregate.Next(foo, "foo", etest.FooEventData{}).Any()
	aggregate.Next(foo, "foo", etest.FooEventData{}).Any()

	aggregate.Next(bar, "foo", etest.FooEventData{}).Any()
	aggregate.Next(bar, "soft_deleted", softDeletedEvent{}).Any()

	r.Save(ctx, foo)
	r.Save(ctx, bar)

	str, errs, err := r.Query(ctx, query.New())
	if err != nil {
		t.Fatalf("Query() failed with %v", err)
	}

	histories, err := streams.Drain(ctx, str, errs)
	if err != nil {
		t.Fatalf("drain histories: %v", err)
	}

	if len(histories) != 1 {
		t.Fatalf("only 1 aggregate should have been returned; got %d", len(histories))
	}

	his := histories[0]

	ref := his.Aggregate()

	if ref.Name != "foo" {
		t.Fatalf("aggregate name of history should be %q; is %q", "foo", ref.Name)
	}

	if ref.ID != foo.AggregateID() {
		t.Fatalf("aggregate id of history should be %q; is %q", foo.AggregateID(), ref.ID)
	}
}

type softDeletedEvent struct{}

func (softDeletedEvent) SoftDelete() bool { return true }

type softRestoredEvent struct{}

func (softRestoredEvent) SoftRestore() bool { return true }
