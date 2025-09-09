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
	"github.com/modernice/goes/helper/streams"
)

var (
	// ErrVersionNotFound is an error returned when a requested aggregate version
	// cannot be found in the event store or snapshot store.
	ErrVersionNotFound = errors.New("version not found")

	// ErrDeleted is an error returned when an aggregate has been soft-deleted and
	// an operation on the deleted aggregate is attempted.
	ErrDeleted = errors.New("aggregate was soft-deleted")
)

// Option configures a [Repository].
type Option func(*Repository)

// Repository persists and loads aggregates from an [event.Store], optionally
// working with snapshots and hooks.
type Repository struct {
	store          event.Store
	snapshots      snapshot.Store
	snapSchedule   snapshot.Schedule
	queryModifiers []func(context.Context, aggregate.Query, event.Query) (event.Query, error)
	beforeInsert   []func(context.Context, aggregate.Aggregate) error
	afterInsert    []func(context.Context, aggregate.Aggregate) error
	onFailedInsert []func(context.Context, aggregate.Aggregate, error) error
	onDelete       []func(context.Context, aggregate.Aggregate) error

	validateConsistency bool
}

// WithSnapshots enables snapshot support.
func WithSnapshots(store snapshot.Store, s snapshot.Schedule) Option {
	if store == nil {
		panic("nil Store")
	}
	return func(r *Repository) {
		r.snapshots = store
		r.snapSchedule = s
	}
}

// ValidateConsistency toggles consistency checks during Save.
func ValidateConsistency(validate bool) Option {
	return func(r *Repository) {
		r.validateConsistency = validate
	}
}

// ModifyQueries appends query modifiers that adjust event queries.
func ModifyQueries(mods ...func(ctx context.Context, q aggregate.Query, prev event.Query) (event.Query, error)) Option {
	return func(r *Repository) {
		r.queryModifiers = append(r.queryModifiers, mods...)
	}
}

// BeforeInsert registers a hook run before events are stored.
func BeforeInsert(fn func(context.Context, aggregate.Aggregate) error) Option {
	return func(r *Repository) {
		r.beforeInsert = append(r.beforeInsert, fn)
	}
}

// AfterInsert registers a hook executed after successful inserts.
func AfterInsert(fn func(context.Context, aggregate.Aggregate) error) Option {
	return func(r *Repository) {
		r.afterInsert = append(r.afterInsert, fn)
	}
}

// OnFailedInsert registers a hook for failed inserts.
func OnFailedInsert(fn func(context.Context, aggregate.Aggregate, error) error) Option {
	return func(r *Repository) {
		r.onFailedInsert = append(r.onFailedInsert, fn)
	}
}

// OnDelete registers a hook run after Delete.
func OnDelete(fn func(context.Context, aggregate.Aggregate) error) Option {
	return func(r *Repository) {
		r.onDelete = append(r.onDelete, fn)
	}
}

// New creates a new Repository instance with the provided event.Store and
// options. The Repository is used for saving, fetching, and deleting aggregates
// while handling snapshots, consistency validation, and various hooks.
func New(store event.Store, opts ...Option) *Repository {
	return newRepository(store, opts...)
}

func newRepository(store event.Store, opts ...Option) *Repository {
	r := &Repository{
		store:               store,
		validateConsistency: true,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Save stores the changes of an Aggregate into the event store and creates a
// snapshot of the Aggregate if the snapshot schedule is met. It validates
// consistency and calls the appropriate hooks before and after inserting
// events. If an error occurs, it calls the OnFailedInsert hook.
func (r *Repository) Save(ctx context.Context, a aggregate.Aggregate) error {
	if r.validateConsistency {
		id, name, version := a.Aggregate()
		ref := aggregate.Ref{Name: name, ID: id}
		if err := aggregate.ValidateConsistency(ref, version, a.AggregateChanges()); err != nil {
			return fmt.Errorf("validate consistency: %w", err)
		}
	}

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

	if c, ok := a.(aggregate.Committer); ok {
		c.Commit()
	}

	if snap {
		if err := r.makeSnapshot(ctx, a); err != nil {
			return fmt.Errorf("make snapshot: %w", err)
		}
	}

	return nil
}

func (r *Repository) makeSnapshot(ctx context.Context, a aggregate.Aggregate) error {
	snap, err := snapshot.New(a)
	if err != nil {
		return err
	}
	if err = r.snapshots.Save(ctx, snap); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

// Fetch retrieves the latest state of the provided aggregate by applying its
// event history. If the aggregate implements snapshot.Target and a snapshot
// store is configured, Fetch loads the latest snapshot and applies events that
// occurred after the snapshot was taken.
func (r *Repository) Fetch(ctx context.Context, a aggregate.Aggregate) error {
	if _, ok := a.(snapshot.Target); ok && r.snapshots != nil {
		return r.fetchLatestWithSnapshot(ctx, a)
	}

	return r.fetch(ctx, a, equery.AggregateVersion(
		version.Min(aggregate.UncommittedVersion(a)+1),
	))
}

func (r *Repository) fetchLatestWithSnapshot(ctx context.Context, a aggregate.Aggregate) error {
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

func (r *Repository) fetch(ctx context.Context, a aggregate.Aggregate, opts ...equery.Option) error {
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

	if err = aggregate.ApplyHistory(a, events); err != nil {
		return fmt.Errorf("apply history: %w", err)
	}

	return nil
}

func (r *Repository) queryEvents(ctx context.Context, q equery.Query) ([]event.Event, error) {
	str, errs, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	out := make([]event.Event, 0, len(str))
	var softDeleted bool
	if err := streams.Walk(ctx, func(evt event.Event) error {
		data := evt.Data()

		if data, ok := data.(aggregate.SoftDeleter); ok && data.SoftDelete() {
			softDeleted = true
		}

		if data, ok := data.(aggregate.SoftRestorer); ok && data.SoftRestore() {
			softDeleted = false
		}

		out = append(out, evt)

		return nil
	}, str, errs); err != nil {
		return out, err
	}

	if softDeleted {
		return out, ErrDeleted
	}

	return out, nil
}

// FetchVersion fetches the specified version of the aggregate from the event
// store and applies its history. It returns ErrVersionNotFound if the requested
// version is not found, and ErrDeleted if the aggregate was soft-deleted.
func (r *Repository) FetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
	if v < 0 {
		v = 0
	}

	if r.snapshots != nil {
		return r.fetchVersionWithSnapshot(ctx, a, v)
	}

	return r.fetchVersion(ctx, a, v)
}

func (r *Repository) fetchVersionWithSnapshot(ctx context.Context, a aggregate.Aggregate, v int) error {
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

func (r *Repository) fetchVersion(ctx context.Context, a aggregate.Aggregate, v int) error {
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

// Delete fetches the aggregate's events from the event store, deletes them, and
// calls OnDelete hooks. It returns an error if the deletion fails or any of the
// OnDelete hooks return an error.
func (r *Repository) Delete(ctx context.Context, a aggregate.Aggregate) error {
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

// Query returns a channel of aggregate.History and a channel of errors by
// executing the provided aggregate.Query. An error is returned if there is an
// issue with constructing the event.Query or querying events from the event
// store.
func (r *Repository) Query(ctx context.Context, q aggregate.Query) (<-chan aggregate.History, <-chan error, error) {
	eq, err := r.makeQuery(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("make query options: %w", err)
	}

	events, errs, err := r.store.Query(ctx, eq)
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	out, outErrors := stream.New(
		ctx,
		events,
		stream.Errors(errs),
		stream.Grouped(true),
		stream.Sorted(true),
	)

	return out, outErrors, nil
}

func (r *Repository) makeQuery(ctx context.Context, aq aggregate.Query) (event.Query, error) {
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

// Use fetches an aggregate, executes the provided function, and saves the
// aggregate. It retries the process if the aggregate is a Retryer and an
// IsRetryable error occurs.
func (r *Repository) Use(ctx context.Context, a aggregate.Aggregate, fn func() error) error {
	var err error

	var trigger RetryTrigger
	var isRetryable IsRetryable

	if rp, ok := a.(Retryer); ok {
		trigger, isRetryable = rp.RetryUse()
	}

	for {
		if err != nil {
			if trigger == nil || isRetryable == nil || !isRetryable(err) {
				return err
			}

			if done := trigger.next(ctx); done != nil {
				return fmt.Errorf("%v: %w", done, err)
			}

			if discarder, ok := a.(ChangeDiscarder); ok {
				discarder.DiscardChanges()
			}
		}

		if err = r.Fetch(ctx, a); err != nil {
			err = fmt.Errorf("fetch aggregate: %w", err)
			continue
		}

		if err = fn(); err != nil {
			continue
		}

		if err = r.Save(ctx, a); err != nil {
			err = fmt.Errorf("save aggregate: %w", err)
			continue
		}

		return nil
	}
}
