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
type Option[D any] interface {
	Apply(*Repository[D])
}

type Repository[D any] struct {
	store          event.Store[D]
	snapshots      snapshot.Store
	snapSchedule   snapshot.Schedule[D]
	queryModifiers []func(context.Context, aggregate.Query, event.Query) (event.Query, error)
	beforeInsert   []func(context.Context, aggregate.Aggregate[D]) error
	afterInsert    []func(context.Context, aggregate.Aggregate[D]) error
	onFailedInsert []func(context.Context, aggregate.Aggregate[D], error) error
	onDelete       []func(context.Context, aggregate.Aggregate[D]) error
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
func WithSnapshots[D any](store snapshot.Store, s snapshot.Schedule[D]) Option[D] {
	if store == nil {
		panic("nil Store")
	}
	return withSnapshots[D]{store, s}
}

type withSnapshots[D any] struct {
	store    snapshot.Store
	schedule snapshot.Schedule[D]
}

func (opt withSnapshots[D]) Apply(r *Repository[D]) {
	r.snapshots = opt.store
	r.snapSchedule = opt.schedule
}

// ModifyQueries returns an Option that adds mods as Query modifiers to a
// Repository. When the Repository builds a Query, it is passed to every
// modifier before the event store is queried.
func ModifyQueries[D any](mods ...func(ctx context.Context, q aggregate.Query, prev event.Query) (event.Query, error)) Option[D] {
	return modifyQueries[D](mods)
}

type modifyQueries[D any] []func(context.Context, aggregate.Query, event.Query) (event.Query, error)

func (opt modifyQueries[D]) Apply(r *Repository[D]) {
	r.queryModifiers = append(r.queryModifiers, opt...)
}

// BeforeInsert returns an Option that adds fn as a hook to a Repository. fn is
// called before the changes to an aggregate are inserted into the event store.
func BeforeInsert[D any](fn func(context.Context, aggregate.Aggregate[D]) error) Option[D] {
	return beforeInsert[D](fn)
}

type beforeInsert[D any] func(context.Context, aggregate.Aggregate[D]) error

func (opt beforeInsert[D]) Apply(r *Repository[D]) {
	r.beforeInsert = append(r.beforeInsert, opt)
}

// AfterInsert returns an Option that adds fn as a hook to a Repository. fn is
// called after the changes to an aggregate are inserted into the event store.
func AfterInsert[D any](fn func(context.Context, aggregate.Aggregate[D]) error) Option[D] {
	return afterInsert[D](fn)
}

type afterInsert[D any] func(context.Context, aggregate.Aggregate[D]) error

func (opt afterInsert[D]) Apply(r *Repository[D]) {
	r.afterInsert = append(r.afterInsert, opt)
}

// OnFailedInsert returns an Option that adds fn as a hook to a Repository. fn
// is called when the Repository fails to insert the changes to an aggregate
// into the event store.
func OnFailedInsert[D any](fn func(context.Context, aggregate.Aggregate[D], error) error) Option[D] {
	return onFailedInsert[D](fn)
}

type onFailedInsert[D any] func(context.Context, aggregate.Aggregate[D], error) error

func (opt onFailedInsert[D]) Apply(r *Repository[D]) {
	r.onFailedInsert = append(r.onFailedInsert, opt)
}

// OnDelete returns an Option that adds fn as a hook to a Repository. fn is
// called after an aggregate has been deleted.
func OnDelete[D any](fn func(context.Context, aggregate.Aggregate[D]) error) Option[D] {
	return onDelete[D](fn)
}

type onDelete[D any] func(context.Context, aggregate.Aggregate[D]) error

func (opt onDelete[D]) Apply(r *Repository[D]) {
	r.onDelete = append(r.onDelete, opt)
}

// New returns an event-sourced Aggregate Repository. It uses the provided Event
// Store to persist and query Aggregates.
func New[D any](store event.Store[D], opts ...Option[D]) *Repository[D] {
	return newRepository(store, opts...)
}

func newRepository[D any](store event.Store[D], opts ...Option[D]) *Repository[D] {
	r := Repository[D]{store: store}
	for _, opt := range opts {
		opt.Apply(&r)
	}
	return &r
}

// Save saves the changes to an Aggregate into the underlying event store and
// flushes its changes afterwards (by calling a.FlushChanges).
func (r *Repository[D]) Save(ctx context.Context, a aggregate.Aggregate[D]) error {
	var snap bool
	if r.snapSchedule != nil && r.snapSchedule.Test(a) {
		snap = true
	}

	for _, fn := range r.beforeInsert {
		if err := fn(ctx, a); err != nil {
			return fmt.Errorf("BeforeInsert: %w", err)
		}
	}

	if err := r.store.Insert(ctx, a.AggregateChanges()...); err != nil {
		for _, fn := range r.onFailedInsert {
			if hookError := fn(ctx, a, err); hookError != nil {
				return fmt.Errorf("OnFailedInsert (%s): %w", err, hookError)
			}
		}

		return fmt.Errorf("insert events: %w", err)
	}

	for _, fn := range r.afterInsert {
		if err := fn(ctx, a); err != nil {
			return fmt.Errorf("AfterInsert: %w", err)
		}
	}

	if c, ok := a.(aggregate.Committer[D]); ok {
		c.Commit()
	}

	if snap {
		if err := r.makeSnapshot(ctx, a); err != nil {
			return fmt.Errorf("make snapshot: %w", err)
		}
	}

	return nil
}

func (r *Repository[D]) makeSnapshot(ctx context.Context, a aggregate.Aggregate[D]) error {
	snap, err := snapshot.NewOf(a)
	if err != nil {
		return err
	}
	if err = r.snapshots.Save(ctx, snap); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

// Fetch fetches the events of the provided Aggregate from the event store and
// applies them onto it to build its current state.
//
// It is allowed to pass an Aggregate that does't have any events in the event
// store yet.
//
// It is also allowed to pass an Aggregate that has already events applied onto
// it. Only events with a version higher than the current version of the passed
// Aggregate are fetched from the event store.
func (r *Repository[D]) Fetch(ctx context.Context, a aggregate.Aggregate[D]) error {
	if _, ok := a.(snapshot.Target); ok && r.snapshots != nil {
		return r.fetchLatestWithSnapshot(ctx, a)
	}

	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(aggregate.UncommittedVersion(a)+1),
	))
}

func (r *Repository[D]) fetchLatestWithSnapshot(ctx context.Context, a aggregate.Aggregate[D]) error {
	id, name, _ := a.Aggregate()

	snap, err := r.snapshots.Latest(ctx, name, id)
	if err != nil || snap == nil {
		return r.fetch(ctx, a, equery.AggregateVersion(
			version.Min(aggregate.UncommittedVersion(a)+1),
		))
	}

	if a, ok := a.(snapshot.Target); !ok {
		return fmt.Errorf("aggregate does not implement %T", a)
	} else {
		if err := snapshot.Unmarshal(snap, a); err != nil {
			return fmt.Errorf("unmarshal snapshot: %w", err)
		}
	}

	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(aggregate.UncommittedVersion(a)+1),
	))
}

func (r *Repository[D]) fetch(ctx context.Context, a aggregate.Aggregate[D], opts ...equery.Option) error {
	id, name, _ := a.Aggregate()

	opts = append([]equery.Option{
		equery.AggregateName(name),
		equery.AggregateID(id),
		equery.SortBy(event.SortAggregateVersion, event.SortAsc),
	}, opts...)

	events, err := r.queryEvents(ctx, equery.New(opts...))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if err = aggregate.ApplyHistory(a, events...); err != nil {
		return fmt.Errorf("apply history: %w", err)
	}

	return nil
}

func (r *Repository[D]) queryEvents(ctx context.Context, q equery.Query) ([]event.Event[D], error) {
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

// FetchVersion does the same as r.Fetch, but only fetches events up until the
// given version v. If the event store has no event for the provided Aggregate
// with the requested version, ErrVersionNotFound is returned.
func (r *Repository[D]) FetchVersion(ctx context.Context, a aggregate.Aggregate[D], v int) error {
	if v < 0 {
		v = 0
	}

	if r.snapshots != nil {
		return r.fetchVersionWithSnapshot(ctx, a, v)
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *Repository[D]) fetchVersionWithSnapshot(ctx context.Context, a aggregate.Aggregate[D], v int) error {
	id, name, _ := a.Aggregate()

	snap, err := r.snapshots.Limit(ctx, name, id, v)
	if err != nil || snap == nil {
		return r.fetchVersion(ctx, a, v)
	}

	if a, ok := a.(snapshot.Target); !ok {
		return fmt.Errorf("aggregate does not implement %T", a)
	} else {
		if err = snapshot.Unmarshal(snap, a); err != nil {
			return fmt.Errorf("unmarshal snapshot: %w", err)
		}
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *Repository[D]) fetchVersion(ctx context.Context, a aggregate.Aggregate[D], v int) error {
	if err := r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(aggregate.UncommittedVersion(a)+1),
		version.Max(v),
	)); err != nil {
		return err
	}

	_, _, av := a.Aggregate()
	if av != v {
		return ErrVersionNotFound
	}

	return nil
}

// Delete deletes an aggregate by deleting its events from the event store.
func (r *Repository[D]) Delete(ctx context.Context, a aggregate.Aggregate[D]) error {
	id, name, _ := a.Aggregate()

	str, errs, err := r.store.Query(ctx, equery.New(
		equery.AggregateName(name),
		equery.AggregateID(id),
	))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	for {
		if str == nil && errs == nil {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			return fmt.Errorf("event stream: %w", err)
		case evt, ok := <-str:
			if !ok {
				str = nil
				break
			}
			if err = r.store.Delete(ctx, evt); err != nil {
				return fmt.Errorf("delete %q event (ID=%s): %w", evt.Name(), evt.ID(), err)
			}
		}
	}

	for _, fn := range r.onDelete {
		if err := fn(ctx, a); err != nil {
			return fmt.Errorf("OnDelete: %w", err)
		}
	}

	return nil
}

// Query queries the event store for events that match the given Query and
// returns a stream of aggregate Histories and errors. Use the returned
// Histories to build the current state of the queried aggregates:
//
//	var r *Repository
//	str, errs, err := r.Query(context.TODO(), query.New(...))
//	// handle err
//	histories, err := aggregate.Drain(context.TODO(), str, errs)
//	// handle err
//	for _, his := range histories {
//		aggregateName := his.AggregateName()
//		aggregateID := his.AggregateID()
//
//		// Create the aggregate from its name and UUID
//		foo := newFoo(aggregateID)
//
//		// Then apply its History
//		his.Apply(foo)
//	}
func (r *Repository[D]) Query(ctx context.Context, q aggregate.Query) (<-chan aggregate.History[D], <-chan error, error) {
	eq, err := r.makeQuery(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("make query options: %w", err)
	}

	events, errs, err := r.store.Query(ctx, eq)
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	out, outErrors := stream.NewOf(
		events,
		stream.Errors[D](errs),
		stream.Grouped[D](true),
		stream.Sorted[D](true),
	)

	return out, outErrors, nil
}

func (r *Repository[D]) makeQuery(ctx context.Context, aq aggregate.Query) (event.Query, error) {
	opts := append(
		query.EventQueryOpts(aq),
		equery.SortByAggregate(),
	)

	var q event.Query = equery.New(opts...)
	var err error
	for _, mod := range r.queryModifiers {
		if q, err = mod(ctx, aq, q); err != nil {
			return q, fmt.Errorf("modify query: %w", err)
		}
	}

	return q, nil
}

// Use is first fetches the aggregate, then calls fn and finally saves the aggregate.
func (r *Repository[D]) Use(ctx context.Context, a aggregate.Aggregate[D], fn func() error) error {
	if err := r.Fetch(ctx, a); err != nil {
		return fmt.Errorf("fetch aggregate: %w", err)
	}

	if err := fn(); err != nil {
		return err
	}

	if err := r.Save(ctx, a); err != nil {
		return fmt.Errorf("save aggregate: %w", err)
	}

	return nil
}
