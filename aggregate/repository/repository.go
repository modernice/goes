package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	equery "github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/helper/streams"
)

var (
	// ErrVersionNotFound is returned when trying to fetch an Aggregate with a
	// version higher than the current version of the Aggregate.
	ErrVersionNotFound = errors.New("version not found")
)

// Option is a repository option.
type Option[ID goes.ID] interface {
	Apply(*Repository[ID])
}

type Repository[ID goes.ID] struct {
	store          event.Store[ID]
	snapshots      snapshot.Store[ID]
	snapSchedule   snapshot.Schedule[ID]
	queryModifiers []func(context.Context, aggregate.Query[ID], event.QueryOf[ID]) (event.QueryOf[ID], error)
	beforeInsert   []func(context.Context, aggregate.AggregateOf[ID]) error
	afterInsert    []func(context.Context, aggregate.AggregateOf[ID]) error
	onFailedInsert []func(context.Context, aggregate.AggregateOf[ID], error) error
	onDelete       []func(context.Context, aggregate.AggregateOf[ID]) error
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
//	var store snapshot.Store[ID]
//	r := repository.New(store, snapshot.Every(3))
//
// The example above will make a Snapshot of an Aggregate every third version of
// the Aggregate.
//
// Aggregates must implement snapshot.Marshaler & snapshot.Unmarshaler in order
// for Snapshots to work.
func WithSnapshots[ID goes.ID](store snapshot.Store[ID], s snapshot.Schedule[ID]) Option[ID] {
	if store == nil {
		panic("nil Store")
	}
	return withSnapshots[ID]{store, s}
}

type withSnapshots[ID goes.ID] struct {
	store    snapshot.Store[ID]
	schedule snapshot.Schedule[ID]
}

func (opt withSnapshots[ID]) Apply(r *Repository[ID]) {
	r.snapshots = opt.store
	r.snapSchedule = opt.schedule
}

// ModifyQueries returns an Option that adds mods as Query modifiers to a
// Repository. When the Repository builds a Query, it is passed to every
// modifier before the event store is queried.
func ModifyQueries[ID goes.ID](mods ...func(ctx context.Context, q aggregate.Query[ID], prev event.QueryOf[ID]) (event.QueryOf[ID], error)) Option[ID] {
	return modifyQueries[ID](mods)
}

type modifyQueries[ID goes.ID] []func(context.Context, aggregate.Query[ID], event.QueryOf[ID]) (event.QueryOf[ID], error)

func (opt modifyQueries[ID]) Apply(r *Repository[ID]) {
	r.queryModifiers = append(r.queryModifiers, opt...)
}

// BeforeInsert returns an Option that adds fn as a hook to a Repository. fn is
// called before the changes to an aggregate are inserted into the event store.
func BeforeInsert[ID goes.ID](fn func(context.Context, aggregate.AggregateOf[ID]) error) Option[ID] {
	return beforeInsert[ID](fn)
}

type beforeInsert[ID goes.ID] func(context.Context, aggregate.AggregateOf[ID]) error

func (opt beforeInsert[ID]) Apply(r *Repository[ID]) {
	r.beforeInsert = append(r.beforeInsert, opt)
}

// AfterInsert returns an Option that adds fn as a hook to a Repository. fn is
// called after the changes to an aggregate are inserted into the event store.
func AfterInsert[ID goes.ID](fn func(context.Context, aggregate.AggregateOf[ID]) error) Option[ID] {
	return afterInsert[ID](fn)
}

type afterInsert[ID goes.ID] func(context.Context, aggregate.AggregateOf[ID]) error

func (opt afterInsert[ID]) Apply(r *Repository[ID]) {
	r.afterInsert = append(r.afterInsert, opt)
}

// OnFailedInsert returns an Option that adds fn as a hook to a Repository. fn
// is called when the Repository fails to insert the changes to an aggregate
// into the event store.
func OnFailedInsert[ID goes.ID](fn func(context.Context, aggregate.AggregateOf[ID], error) error) Option[ID] {
	return onFailedInsert[ID](fn)
}

type onFailedInsert[ID goes.ID] func(context.Context, aggregate.AggregateOf[ID], error) error

func (opt onFailedInsert[ID]) Apply(r *Repository[ID]) {
	r.onFailedInsert = append(r.onFailedInsert, opt)
}

// OnDelete returns an Option that adds fn as a hook to a Repository. fn is
// called after an aggregate has been deleted.
func OnDelete[ID goes.ID](fn func(context.Context, aggregate.AggregateOf[ID]) error) Option[ID] {
	return onDelete[ID](fn)
}

type onDelete[ID goes.ID] func(context.Context, aggregate.AggregateOf[ID]) error

func (opt onDelete[ID]) Apply(r *Repository[ID]) {
	r.onDelete = append(r.onDelete, opt)
}

// New returns an event-sourced Aggregate Repository. It uses the provided Event
// Store to persist and query Aggregates.
func New[ID goes.ID](store event.Store[ID], opts ...Option[ID]) *Repository[ID] {
	r := Repository[ID]{store: store}
	for _, opt := range opts {
		opt.Apply(&r)
	}
	return &r
}

// Save saves the changes to an Aggregate into the underlying event store and
// flushes its changes afterwards (by calling a.FlushChanges).
func (r *Repository[ID]) Save(ctx context.Context, a aggregate.AggregateOf[ID]) error {
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

	if c, ok := a.(aggregate.Committer[ID]); ok {
		c.Commit()
	}

	if snap {
		if err := r.makeSnapshot(ctx, a); err != nil {
			return fmt.Errorf("make snapshot: %w", err)
		}
	}

	return nil
}

func (r *Repository[ID]) makeSnapshot(ctx context.Context, a aggregate.AggregateOf[ID]) error {
	snap, err := snapshot.New(a)
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
func (r *Repository[ID]) Fetch(ctx context.Context, a aggregate.AggregateOf[ID]) error {
	if _, ok := a.(snapshot.Target); ok && r.snapshots != nil {
		return r.fetchLatestWithSnapshot(ctx, a)
	}

	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(aggregate.UncommittedVersion(a)+1),
	))
}

func (r *Repository[ID]) fetchLatestWithSnapshot(ctx context.Context, a aggregate.AggregateOf[ID]) error {
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

func (r *Repository[ID]) fetch(ctx context.Context, a aggregate.AggregateOf[ID], opts ...equery.Option) error {
	id, name, _ := a.Aggregate()

	opts = append([]equery.Option{
		equery.AggregateName(name),
		equery.AggregateID(id),
		equery.SortBy(event.SortAggregateVersion, event.SortAsc),
	}, opts...)

	events, err := r.queryEvents(ctx, equery.New[ID](opts...))
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if err = aggregate.ApplyHistory[any, ID](a, events); err != nil {
		return fmt.Errorf("apply history: %w", err)
	}

	return nil
}

func (r *Repository[ID]) queryEvents(ctx context.Context, q equery.Query[ID]) ([]event.Of[any, ID], error) {
	str, errs, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	events, err := streams.Drain(ctx, str, errs)
	if err != nil {
		return events, fmt.Errorf("stream: %w", err)
	}

	return events, nil
}

// FetchVersion does the same as r.Fetch, but only fetches events up until the
// given version v. If the event store has no event for the provided Aggregate
// with the requested version, ErrVersionNotFound is returned.
func (r *Repository[ID]) FetchVersion(ctx context.Context, a aggregate.AggregateOf[ID], v int) error {
	if v < 0 {
		v = 0
	}

	if r.snapshots != nil {
		return r.fetchVersionWithSnapshot(ctx, a, v)
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *Repository[ID]) fetchVersionWithSnapshot(ctx context.Context, a aggregate.AggregateOf[ID], v int) error {
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

func (r *Repository[ID]) fetchVersion(ctx context.Context, a aggregate.AggregateOf[ID], v int) error {
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
func (r *Repository[ID]) Delete(ctx context.Context, a aggregate.AggregateOf[ID]) error {
	id, name, _ := a.Aggregate()

	str, errs, err := r.store.Query(ctx, equery.New[ID](
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
//	histories, err := streams.Drain(context.TODO(), str, errs)
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
func (r *Repository[ID]) Query(ctx context.Context, q aggregate.Query[ID]) (<-chan aggregate.HistoryOf[ID], <-chan error, error) {
	eq, err := r.makeQuery(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("make query options: %w", err)
	}

	events, errs, err := r.store.Query(ctx, eq)
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

func (r *Repository[ID]) makeQuery(ctx context.Context, aq aggregate.Query[ID]) (event.QueryOf[ID], error) {
	opts := append(
		query.EventQueryOpts(aq),
		equery.SortByAggregate(),
	)

	var q event.QueryOf[ID] = equery.New[ID](opts...)
	var err error
	for _, mod := range r.queryModifiers {
		if q, err = mod(ctx, aq, q); err != nil {
			return q, fmt.Errorf("modify query: %w", err)
		}
	}

	return q, nil
}

// Use first fetches the Aggregate a, then calls fn(a) and finally saves the aggregate.
func (r *Repository[ID]) Use(ctx context.Context, a aggregate.AggregateOf[ID], fn func() error) error {
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
