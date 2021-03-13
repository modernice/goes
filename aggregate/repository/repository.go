package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

var (
	// ErrVersionNotFound is returned when trying to fetch an Aggregate with a
	// version higher than the current version of the Aggregate.
	ErrVersionNotFound = errors.New("version not found")
)

// Option is a repository option.
type Option func(*repository)

type repository struct {
	store     event.Store
	snapshots snapshot.Store
}

// WithSnapshots returns an Option that add a Snapshot Store to a Repository.
//
// A Repository using Snapshots will use those as the base state when fetching a
// single Aggregate.
//
// Aggregates must implement snapshot.Marshaler & snapshot.Unmarshaler in order
// for Snapshots to work.
func WithSnapshots(s snapshot.Store) Option {
	if s == nil {
		panic("nil Store")
	}
	return func(r *repository) {
		r.snapshots = s
	}
}

// New returns an event-sourced Aggregate Repository. It uses the provided Event
// Store to persist and query Aggregates.
func New(store event.Store, opts ...Option) aggregate.Repository {
	r := repository{store: store}
	for _, opt := range opts {
		opt(&r)
	}
	return &r
}

func (r *repository) Save(ctx context.Context, a aggregate.Aggregate) error {
	if err := r.store.Insert(ctx, a.AggregateChanges()...); err != nil {
		return fmt.Errorf("insert events: %w", err)
	}
	a.FlushChanges()
	return nil
}

func (r *repository) Fetch(ctx context.Context, a aggregate.Aggregate) error {
	if r.snapshots != nil {
		return r.fetchLatestWithSnapshot(ctx, a)
	}
	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
	))
}

func (r *repository) fetchLatestWithSnapshot(ctx context.Context, a aggregate.Aggregate) error {
	snap, err := r.snapshots.Latest(ctx, a.AggregateName(), a.AggregateID())
	if err != nil || snap == nil {
		return r.fetch(ctx, a, equery.AggregateVersion(
			version.Min(a.AggregateVersion()+1),
		))
	}

	if err := snapshot.Unmarshal(snap, a); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
	))
}

func (r *repository) fetch(ctx context.Context, a aggregate.Aggregate, opts ...equery.Option) error {
	opts = append([]equery.Option{
		equery.AggregateName(a.AggregateName()),
		equery.AggregateID(a.AggregateID()),
		equery.SortBy(event.SortAggregateVersion, event.SortAsc),
	}, opts...)

	events, err := r.queryEvents(ctx, equery.New(opts...))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if err = aggregate.ApplyHistory(a, events...); err != nil {
		return fmt.Errorf("apply events: %w", err)
	}

	return nil
}

func (r *repository) queryEvents(ctx context.Context, q equery.Query) ([]event.Event, error) {
	str, errs, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	events, err := event.Drain(ctx, str, errs)
	if err != nil {
		return events, fmt.Errorf("stream: %w", err)
	}

	return events, nil
}

func (r *repository) FetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
	if v < 0 {
		v = 0
	}

	if r.snapshots != nil {
		return r.fetchVersionWithSnapshot(ctx, a, v)
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *repository) fetchVersionWithSnapshot(ctx context.Context, a aggregate.Aggregate, v int) error {
	snap, err := r.snapshots.Limit(ctx, a.AggregateName(), a.AggregateID(), v)
	if err != nil || snap == nil {
		return r.fetchVersion(ctx, a, v)
	}

	if err = snapshot.Unmarshal(snap, a); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *repository) fetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
	if err := r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(a.AggregateVersion()+1),
		version.Max(v),
	)); err != nil {
		return err
	}

	if a.AggregateVersion() != v {
		return ErrVersionNotFound
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, a aggregate.Aggregate) error {
	str, errs, err := r.store.Query(ctx, equery.New(
		equery.AggregateName(a.AggregateName()),
		equery.AggregateID(a.AggregateID()),
	))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errs:
			if !ok {
				return nil
			}
			return fmt.Errorf("event stream: %w", err)
		case evt, ok := <-str:
			if !ok {
				return nil
			}
			if err = r.store.Delete(ctx, evt); err != nil {
				return fmt.Errorf("delete %q event (ID=%s): %w", evt.Name(), evt.ID(), err)
			}
		}
	}
}

func (r *repository) Query(ctx context.Context, q aggregate.Query) (<-chan aggregate.History, <-chan error, error) {
	opts := makeQueryOptions(q)
	events, errs, err := r.store.Query(ctx, equery.New(opts...))
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}
	out, outErrors := stream.New(
		events,
		stream.Errors(errs),
		stream.Grouped(true),
		stream.Sorted(true),
	)
	return out, outErrors, nil
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
