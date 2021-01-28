package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	estream "github.com/modernice/goes/event/stream"
)

type repository struct {
	store   event.Store
	factory aggregate.Factory
}

// New return a Repository for aggregates. The Repository uses the provided
// store to query the events needed to build the state of aggregates and to
// insert the aggregate changes in form of events into the Store.
func New(store event.Store, fac aggregate.Factory) aggregate.Repository {
	return &repository{
		store:   store,
		factory: fac,
	}
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

	events, err := estream.All(ctx, cur)
	if err != nil {
		return events, fmt.Errorf("stream: %w", err)
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
		return fmt.Errorf("stream: %w", err)
	}

	return nil
}

func (r *repository) Query(ctx context.Context, q aggregate.Query) (aggregate.Stream, error) {
	opts := makeQueryOptions(q)
	es, err := r.store.Query(ctx, equery.New(opts...))
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	return stream.FromEvents(
		es,
		stream.Factory(r.factory),
		stream.Grouped(true),
		stream.Sorted(true),
	), nil
}

func buildAggregate(a aggregate.Aggregate, events ...event.Event) error {
	for _, evt := range events {
		a.ApplyEvent(evt)
	}
	a.TrackChange(events...)
	a.FlushChanges()
	return nil
}

func makeQueryOptions(q aggregate.Query) []equery.Option {
	opts := append(
		query.EventQueryOpts(q),
		equery.SortByMulti(
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	)
	return opts
}
