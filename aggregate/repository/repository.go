package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/aggregate/tagging"
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
	store        event.Store
	snapshots    snapshot.Store
	snapSchedule snapshot.Schedule
	tags         tagging.Store
}

// WithSnapshots returns an Option that add a Snapshot Store to a Repository.
//
// A Repository that has a Snapshot Store will fetch the latest valid Snapshot
// for an Aggregate before fetching the necessary Events to reconstruct the
// state of the Agrgegate.
//
// An optional Snapshot Schedule can be provided to instruct the Repository to
// make and save Snapshots into the Snapshot Store when appropriate:
//
//	var store snapshot.Store
//	r := repository.New(store, snapshot.Every(3))
//
// The example above will make a Snapshot of an Aggregate every third version of
// the Aggregate.
//
// Aggregates must implement snapshot.Marshaler & snapshot.Unmarshaler in order
// for Snapshots to work.
func WithSnapshots(store snapshot.Store, s snapshot.Schedule) Option {
	if store == nil {
		panic("nil Store")
	}
	return func(r *repository) {
		r.snapshots = store
		r.snapSchedule = s
	}
}

// WithTags returns an Option that adds a tagging Store to a Repository.
func WithTags(store tagging.Store) Option {
	if store == nil {
		panic("nil Store")
	}
	return func(r *repository) {
		r.tags = store
	}
}

// New returns an event-sourced Aggregate Repository. It uses the provided Event
// Store to persist and query Aggregates.
func New(store event.Store, opts ...Option) aggregate.Repository {
	return newRepository(store, opts...)
}

func newRepository(store event.Store, opts ...Option) *repository {
	r := repository{store: store}
	for _, opt := range opts {
		opt(&r)
	}
	return &r
}

type tagger interface {
	Tags() []string
}

func (r *repository) Save(ctx context.Context, a aggregate.Aggregate) error {
	var snap bool
	if r.snapSchedule != nil && r.snapSchedule.Test(a) {
		snap = true
	}

	var rollbackTagUpdate func(context.Context) error
	if r.tags != nil {
		if tagger, ok := a.(tagger); ok {
			oldTags, err := r.tags.Tags(ctx, a.AggregateName(), a.AggregateID())
			if err != nil {
				return fmt.Errorf("fetch current tags: %w", err)
			}

			if err := r.tags.Update(ctx, a.AggregateName(), a.AggregateID(), tagger.Tags()); err != nil {
				return fmt.Errorf("update tags: %w", err)
			}

			rollbackTagUpdate = func(ctx context.Context) error {
				return r.tags.Update(ctx, a.AggregateName(), a.AggregateID(), oldTags)
			}
		}
	}

	if err := r.store.Insert(ctx, a.AggregateChanges()...); err != nil {
		if rollbackTagUpdate != nil {
			if rollbackError := rollbackTagUpdate(ctx); rollbackError != nil {
				return fmt.Errorf("rollback tags after failing to insert events (%s): %w", err, rollbackError)
			}
		}
		return fmt.Errorf("insert events: %w", err)
	}
	a.FlushChanges()

	if snap {
		if err := r.makeSnapshot(ctx, a); err != nil {
			return fmt.Errorf("make snapshot: %w", err)
		}
	}

	return nil
}

func (r *repository) makeSnapshot(ctx context.Context, a aggregate.Aggregate) error {
	snap, err := snapshot.New(a)
	if err != nil {
		return err
	}
	if err = r.snapshots.Save(ctx, snap); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
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

			if r.tags != nil {
				if _, ok := a.(tagger); ok {
					if err := r.tags.Update(ctx, a.AggregateName(), a.AggregateID(), []string{}); err != nil {
						return fmt.Errorf("update tags: %w", err)
					}
				}
			}
		}
	}
}

func (r *repository) Query(ctx context.Context, q aggregate.Query) (<-chan aggregate.History, <-chan error, error) {
	opts, shouldRun, err := r.makeQueryOptions(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("make query options: %w", err)
	}

	if !shouldRun {
		out := make(chan aggregate.History)
		errs := make(chan error)
		close(out)
		close(errs)
		return out, errs, nil
	}

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

func (r *repository) makeQueryOptions(ctx context.Context, q aggregate.Query) ([]equery.Option, bool, error) {
	opts := append(
		query.EventQueryOpts(q),
		equery.SortByAggregate(),
	)

	tags := q.Tags()

	if r.tags == nil || len(tags) == 0 {
		return opts, true, nil
	}

	tagged, err := r.tags.TaggedWith(ctx, tags...)
	if err != nil {
		return opts, true, fmt.Errorf("get tagged (%v) aggregates: %w", tags, err)
	}

	if len(tagged) == 0 {
		return opts, false, nil
	}

	for _, tagged := range tagged {
		opts = append(opts, equery.Aggregate(tagged.Name, tagged.ID))
	}

	return opts, true, nil
}
