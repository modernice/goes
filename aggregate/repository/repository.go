package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/cursor"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

type repository struct {
	store event.Store
}

// New return a Repository for aggregates. The Repository uses the provided
// store to query the events needed to build the state of aggregates and to
// insert the aggregate changes in form of events into the Store.
func New(store event.Store) aggregate.Repository {
	return &repository{store}
}

// Save saves the changes of Aggregate a into the event store. If the underlying
// event store fails to insert the events, Save tries to rollback the changes by
// deleting the successfully stored events from the store and returns a
// *SaveError which contains the causing insert error, the rolled back events
// and the rollback error for those event.
func (r *repository) Save(ctx context.Context, a aggregate.Aggregate) error {
	changes := a.AggregateChanges()
	if err := r.store.Insert(ctx, changes...); err != nil {
		return &SaveError{
			Aggregate: a,
			Err:       err,
			Rollbacks: r.rollbackSave(ctx, a),
		}
	}
	a.FlushChanges()
	return nil
}

func (r *repository) rollbackSave(ctx context.Context, a aggregate.Aggregate) SaveRollbacks {
	var mux sync.Mutex
	var wg sync.WaitGroup
	events := a.AggregateChanges()
	wg.Add(len(events))
	errs := make([]error, len(events))
	for i, evt := range events {
		i := i
		evt := evt
		go func() {
			defer wg.Done()
			if err := r.store.Delete(ctx, evt); err != nil {
				mux.Lock()
				errs[i] = fmt.Errorf("delete event %q: %w", evt.ID(), err)
				mux.Unlock()
			}
		}()
	}
	wg.Wait()

	rbs := make(SaveRollbacks, len(errs))
	for i, err := range errs {
		rbs[i].Event = events[i]
		rbs[i].Err = err
	}

	return rbs
}

func (r *repository) Fetch(ctx context.Context, a aggregate.Aggregate) error {
	return r.fetch(ctx, a, query.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
	))
}

func (r *repository) fetch(ctx context.Context, a aggregate.Aggregate, opts ...query.Option) error {
	opts = append([]query.Option{
		query.AggregateName(a.AggregateName()),
		query.AggregateID(a.AggregateID()),
		query.SortBy(event.SortTime, event.SortAsc),
	}, opts...)

	events, err := r.queryEvents(ctx, query.New(opts...))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if err := buildAggregate(a, events...); err != nil {
		return fmt.Errorf("finalize: %w", err)
	}

	return nil
}

func (r *repository) queryEvents(ctx context.Context, q query.Query) ([]event.Event, error) {
	cur, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	events, err := cursor.All(ctx, cur)
	if err != nil {
		return events, fmt.Errorf("cursor: %w", err)
	}

	return events, nil
}

func (r *repository) FetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
	return r.fetch(ctx, a, query.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
		version.Max(v),
	))
}

func buildAggregate(a aggregate.Aggregate, events ...event.Event) error {
	for _, evt := range events {
		a.ApplyEvent(evt)
	}
	if err := a.TrackChange(events...); err != nil {
		return fmt.Errorf("track change: %w", err)
	}
	a.FlushChanges()
	return nil
}
