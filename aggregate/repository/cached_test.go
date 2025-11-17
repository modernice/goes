package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/internal"
)

func TestCachedRepository_Fetch(t *testing.T) {
	base := repository.New(eventstore.New())

	var constructed int
	typedBase := repository.Typed(base, func(id uuid.UUID) *aggregate.Base {
		// constructor is called once with uuid.Nil when creating the
		// TypedRepository to extract the aggregate name.
		if id != uuid.Nil {
			constructed++
		}
		return aggregate.New("foo", id)
	})

	cached := repository.Cached(typedBase)

	foo := aggregate.New("foo", internal.NewUUID())
	aggregate.Next(foo, "foo.foo", "foobar")
	aggregate.Next(foo, "foo.bar", "barbaz")

	if err := typedBase.Save(context.Background(), foo); err != nil {
		t.Fatalf("save aggregate: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := cached.Fetch(context.Background(), foo.AggregateID()); err != nil {
			t.Fatalf("fetch aggregate: %v", err)
		}
	}

	if constructed != 1 {
		t.Errorf("constructed %d aggregates; want 1", constructed)
	}
}
