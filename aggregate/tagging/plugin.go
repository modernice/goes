package tagging

//go:generate mockgen -source=plugin.go -destination=./mock_tagging/plugin.go

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
)

// A Store persists tags of aggregates.
type Store interface {
	// Update updates the tags of an aggregate.
	Update(_ context.Context, aggregateName string, aggregateID uuid.UUID, tags []string) error

	// Tags returns the tags of an aggregate.
	Tags(context.Context, string, uuid.UUID) ([]string, error)

	// TaggesWith returns the Aggregates that have the given tags.
	TaggedWith(context.Context, ...string) ([]Aggregate, error)
}

// Aggregate references an aggregate by its name and UUID.
type Aggregate struct {
	Name string
	ID   uuid.UUID
}

// Plugin returns a repository Option that can be passed to an aggregate
// repository to enable tagging for that repository.
func Plugin(store Store) repository.Option {
	return &plugin{
		store:    store,
		rollback: make(map[Aggregate][]string),
	}
}

type plugin struct {
	store Store

	mux      sync.RWMutex
	rollback map[Aggregate][]string
}

type tagger interface {
	Tags() []string
}

func (p *plugin) Apply(r *repository.Repository) {
	// Update the tags of an aggregate before its changes are inserted into the
	// event store.
	repository.BeforeInsert(func(ctx context.Context, a aggregate.Aggregate) error {
		tagger, ok := a.(tagger)
		if !ok {
			return nil
		}

		oldTags, err := p.store.Tags(ctx, a.AggregateName(), a.AggregateID())
		if err != nil {
			return fmt.Errorf("get current tags: %w", err)
		}

		if err := p.store.Update(ctx, a.AggregateName(), a.AggregateID(), tagger.Tags()); err != nil {
			return fmt.Errorf("update tags: %w", err)
		}

		p.mux.Lock()
		defer p.mux.Unlock()

		p.rollback[Aggregate{
			Name: a.AggregateName(),
			ID:   a.AggregateID(),
		}] = oldTags

		return nil
	}).Apply(r)

	// If the event store fails to insert events, rollback the tag update.
	repository.OnFailedInsert(func(ctx context.Context, a aggregate.Aggregate, _ error) error {
		tagger, ok := a.(tagger)
		if !ok {
			return nil
		}

		p.mux.Lock()
		agg := Aggregate{
			Name: a.AggregateName(),
			ID:   a.AggregateID(),
		}
		tags := p.rollback[agg]
		delete(p.rollback, agg)
		p.mux.Unlock()

		if err := p.store.Update(ctx, a.AggregateName(), a.AggregateID(), tags); err != nil {
			return fmt.Errorf("rollback tags (%v -> %v): %w", tagger.Tags(), tags, err)
		}

		return nil
	}).Apply(r)

	repository.AfterInsert(func(_ context.Context, a aggregate.Aggregate) error {
		p.mux.Lock()
		delete(p.rollback, Aggregate{
			Name: a.AggregateName(),
			ID:   a.AggregateID(),
		})
		p.mux.Unlock()
		return nil
	}).Apply(r)

	repository.OnDelete(func(ctx context.Context, a aggregate.Aggregate) error {
		if err := p.store.Update(ctx, a.AggregateName(), a.AggregateID(), nil); err != nil {
			return fmt.Errorf("delete tags: %w", err)
		}
		return nil
	}).Apply(r)
}
