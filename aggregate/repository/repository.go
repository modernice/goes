package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	ecursor "github.com/modernice/goes/event/cursor"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

type repository struct {
	store event.Store
}

type cursor struct {
	events     chan event.Event
	aggregates chan aggregate.Aggregate
}

// New return a Repository for aggregates. The Repository uses the provided
// store to query the events needed to build the state of aggregates and to
// insert the aggregate changes in form of events into the Store.
func New(store event.Store) aggregate.Repository {
	return &repository{store}
}

// Save saves the changes of Aggregate a into the event store.
//
// If the underlying event store fails to insert the events, Save tries to
// rollback the changes by deleting the successfully stored events from the
// store and returns a *SaveError which contains the insertion error, the rolled
// back events and the rollback (deletion) errors for those events.
//
// TODO: Figure something out to ensure aggregates aren't being corrupted by a
// failed rollback
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
	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
	))
}

func (r *repository) fetch(ctx context.Context, a aggregate.Aggregate, opts ...equery.Option) error {
	opts = append([]equery.Option{
		equery.AggregateName(a.AggregateName()),
		equery.AggregateID(a.AggregateID()),
		equery.SortBy(event.SortTime, event.SortAsc),
	}, opts...)

	events, err := r.queryEvents(ctx, equery.New(opts...))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if err := buildAggregate(a, events...); err != nil {
		return fmt.Errorf("finalize: %w", err)
	}

	return nil
}

func (r *repository) queryEvents(ctx context.Context, q equery.Query) ([]event.Event, error) {
	cur, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	events, err := ecursor.All(ctx, cur)
	if err != nil {
		return events, fmt.Errorf("cursor: %w", err)
	}

	return events, nil
}

func (r *repository) FetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
		version.Max(v),
	))
}

func (r *repository) Delete(ctx context.Context, a aggregate.Aggregate) error {
	cur, err := r.store.Query(ctx, equery.New(
		equery.AggregateName(a.AggregateName()),
		equery.AggregateID(a.AggregateID()),
	))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		evt := cur.Event()
		if err := r.store.Delete(ctx, evt); err != nil {
			return fmt.Errorf("delete %q event (ID=%s): %w", evt.Name(), evt.ID(), err)
		}
	}

	if cur.Err() != nil {
		return fmt.Errorf("cursor: %w", err)
	}

	return nil
}

func (r *repository) Query(ctx context.Context, q aggregate.Query) (aggregate.Cursor, error) {
	opts := makeQueryOptions(q)

	cur, err := r.store.Query(ctx, equery.New(opts...))
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer cur.Close(ctx)

	return newCursor(ctx, cur), nil
}

func (c *cursor) Next(ctx context.Context) bool {
	return false
}

func (c *cursor) Aggregate() aggregate.Aggregate {
	return nil
}

func (c *cursor) Err() error {
	return nil
}

func (c *cursor) Close(ctx context.Context) error {
	return nil
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

func makeQueryOptions(q aggregate.Query) []equery.Option {
	var opts []equery.Option
	opts = withNameFilter(opts, q)
	return opts
}

func withNameFilter(opts []equery.Option, q aggregate.Query) []equery.Option {
	names := q.Names()
	if len(names) == 0 {
		return opts
	}
	return append(opts, equery.AggregateName(names...))
}

func newCursor(ctx context.Context, cur event.Cursor) *cursor {
	return nil
}
