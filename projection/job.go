package projection

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/helper/streams"
)

var (
	// ErrAggregateNotFound reports that the requested aggregate was not
	// present in the job's events.
	ErrAggregateNotFound = errors.New("aggregate not found in events")
)

// Job wraps a set of events to be applied to projections.
type Job interface {
	context.Context

	// Events streams the job's events, optionally filtered in-memory.
	Events(context.Context, ...event.Query) (<-chan event.Event, <-chan error, error)

	// EventsOf streams events belonging to the named aggregates.
	EventsOf(context.Context, ...string) (<-chan event.Event, <-chan error, error)

	// EventsFor streams the events that would be applied to target.
	EventsFor(context.Context, Target[any]) (<-chan event.Event, <-chan error, error)

	// Aggregates returns unique aggregate references extracted from the events.
	Aggregates(context.Context, ...string) (<-chan aggregate.Ref, <-chan error, error)

	// Aggregate returns the id of the first aggregate with the given name.
	Aggregate(context.Context, string) (uuid.UUID, error)

	// Apply applies the job to target.
	Apply(context.Context, Target[any], ...ApplyOption) error
}

// JobOption configures a [Job].
type JobOption func(*job)

type job struct {
	context.Context

	query event.Query

	// If provided, will be used within the `Aggregates()` and `Aggregate()` methods.
	aggregateQuery event.Query

	beforeEvent []func(context.Context, event.Event) ([]event.Event, error)
	filter      []event.Query
	reset       bool
	cache       *queryCache
}

// WithFilter adds in-memory filters to a Job.
func WithFilter(queries ...event.Query) JobOption {
	return func(j *job) {
		j.filter = append(j.filter, queries...)
	}
}

// WithReset makes the job reset targets before applying events.
func WithReset() JobOption {
	return func(j *job) {
		j.reset = true
	}
}

// WithAggregateQuery overrides the query used by [Job.Aggregates] and
// [Job.Aggregate].
func WithAggregateQuery(q event.Query) JobOption {
	return func(j *job) {
		j.aggregateQuery = q
	}
}

// WithBeforeEvent inserts events returned by fns before each streamed event.
func WithBeforeEvent(fns ...func(context.Context, event.Event) ([]event.Event, error)) JobOption {
	return func(j *job) {
		j.beforeEvent = append(j.beforeEvent, fns...)
	}
}

// NewJob builds a Job that queries events from store using q.
func NewJob(ctx context.Context, store event.Store, q event.Query, opts ...JobOption) Job {
	j := job{
		Context: ctx,
		query:   q,
		cache:   newQueryCache(store),
	}
	for _, opt := range opts {
		opt(&j)
	}
	if j.query == nil {
		j.query = query.New()
	}
	return &j
}

// Events streams queried events. Additional filters are applied in-memory.
func (j *job) Events(ctx context.Context, filter ...event.Query) (<-chan event.Event, <-chan error, error) {
	return j.queryEvents(ctx, j.query, filter...)
}

func (j *job) queryEvents(ctx context.Context, q event.Query, filter ...event.Query) (<-chan event.Event, <-chan error, error) {
	str, errs, err := j.runQuery(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	if len(j.beforeEvent) > 0 {
		str, errs = j.applyBeforeEvent(ctx, str, errs)
	}

	if filter = append(j.filter, filter...); len(filter) > 0 {
		str = event.Filter(str, filter...)
	}

	return str, errs, nil
}

func (j *job) applyBeforeEvent(ctx context.Context, events <-chan event.Event, errs <-chan error) (<-chan event.Event, <-chan error) {
	outErrs := make(chan error)
	fail := func(err error) {
		select {
		case <-ctx.Done():
		case outErrs <- err:
		}
	}

	for _, before := range j.beforeEvent {
		events = streams.BeforeContext(ctx, events, func(evt event.Event) []event.Event {
			add, err := before(ctx, evt)
			if err != nil {
				fail(fmt.Errorf("before %q event: %w", evt.Name(), err))
				return nil
			}
			return add
		})
	}

	out := make(chan event.Event)
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		defer close(out)
		for evt := range events {
			out <- evt
		}
	}()

	go func() {
		defer close(outErrs)
		for err := range errs {
			outErrs <- err
		}
		<-eventsDone
	}()

	return out, outErrs
}

// EventsOf streams events of the specified aggregate names.
func (j *job) EventsOf(ctx context.Context, aggregateName ...string) (<-chan event.Event, <-chan error, error) {
	if len(aggregateName) == 0 {
		return j.Events(ctx)
	}
	return j.Events(ctx, query.New(query.AggregateName(aggregateName...)))
}

// EventsFor streams the events that would be applied to target when calling
// [Job.Apply].
func (j *job) EventsFor(ctx context.Context, target Target[any]) (<-chan event.Event, <-chan error, error) {
	q := j.query

	if progressor, isProgressor := target.(ProgressAware); isProgressor {
		progressTime, _ := progressor.Progress()
		if !progressTime.IsZero() {
			// Why subtract a nanosecond and return possibly already applied
			// events? Because multiple events can have the same time, and we
			// want to ensure that we don't accidentally exclude events that
			// haven't been applied yet. The Apply and ApplyStream functions
			// ensure that an event is not applied twice to a projection that
			// implements ProgressAware.
			q = query.Merge(q, query.New(query.Time(
				time.After(progressTime.Add(-stdtime.Nanosecond))),
			))
		}
	}

	return j.queryEvents(ctx, q)
}

// Aggregates extracts unique aggregate references from the job's events.
func (j *job) Aggregates(ctx context.Context, names ...string) (<-chan aggregate.Ref, <-chan error, error) {
	var (
		events <-chan event.Event
		errs   <-chan error
		err    error
	)

	if j.aggregateQuery != nil {
		var filters []event.Query
		if len(names) > 0 {
			filters = append(filters, query.New(query.AggregateName(names...)))
		}
		events, errs, err = j.queryEvents(ctx, j.aggregateQuery, filters...)
	} else {
		events, errs, err = j.EventsOf(ctx, names...)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	out := make(chan aggregate.Ref)
	found := make(map[aggregate.Ref]struct{})

	go func() {
		defer close(out)
		for evt := range events {
			id, name, _ := evt.Aggregate()
			ref := aggregate.Ref{
				Name: name,
				ID:   id,
			}

			if _, ok := found[ref]; ok {
				continue
			}
			found[ref] = struct{}{}

			select {
			case <-ctx.Done():
				return
			case out <- ref:
			}
		}
	}()

	return out, errs, nil
}

// Aggregate returns the id of the first aggregate with the given name or
// ErrAggregateNotFound.
func (j *job) Aggregate(ctx context.Context, name string) (uuid.UUID, error) {
	tuples, errs, err := j.Aggregates(ctx, name)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID

	done := errors.New("done")
	if err := streams.Walk(ctx, func(t aggregate.Ref) error {
		if t.Name == name {
			id = t.ID
			return done
		}
		return nil
	}, tuples, errs); !errors.Is(err, done) {
		return uuid.Nil, err
	}

	if id == uuid.Nil {
		return uuid.Nil, ErrAggregateNotFound
	}

	return id, nil
}

// Apply feeds the job's events into target. It may run concurrently on multiple
// projections.
func (j *job) Apply(ctx context.Context, target Target[any], opts ...ApplyOption) error {
	if j.reset {
		if progressor, isProgressor := target.(ProgressAware); isProgressor {
			progressor.SetProgress(stdtime.Time{})
		}

		if resetter, isResetter := target.(Resetter); isResetter {
			resetter.Reset()
		}
	}

	events, errs, err := j.EventsFor(ctx, target)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		ApplyStream(target, events, opts...)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-errs:
			if ok {
				return err
			}
			errs = nil
		case <-done:
			return nil
		}
	}
}

func (j *job) runQuery(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	return j.cache.run(ctx, q)
}

type queryCache struct {
	store event.Store

	locksMux sync.Mutex
	locks    map[[32]byte]*sync.Mutex

	cacheMux sync.RWMutex
	cache    map[[32]byte][]event.Event
}

func newQueryCache(store event.Store) *queryCache {
	return &queryCache{
		store: store,
		locks: make(map[[32]byte]*sync.Mutex),
		cache: make(map[[32]byte][]event.Event),
	}
}

func (c *queryCache) run(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	hash := hashQuery(q)

	events, ok := c.cached(hash, true)
	if ok {
		out, errs := eventStream(ctx, events)
		return out, errs, nil
	}

	// Prevent the same query from being run multiple times.
	// If the same query is currently being run, wait for it to be finished so
	// we can use the cached result.
	unlock := c.acquireQueryLock(hash)
	defer unlock()

	// Check again if the query was cached by another run.
	if events, ok = c.cached(hash, false); ok {
		out, errs := eventStream(ctx, events)
		return out, errs, nil
	}

	str, errs, err := c.store.Query(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	return c.intercept(ctx, str, hash), errs, nil
}

func (c *queryCache) cached(hash [32]byte, lock bool) ([]event.Event, bool) {
	var events []event.Event

	if lock {
		c.cacheMux.RLock()
		defer c.cacheMux.RUnlock()
	}

	if cached, ok := c.cache[hash]; ok {
		events = make([]event.Event, len(cached))
		copy(events, cached)
		return events, true
	}

	return events, false
}

func (c *queryCache) acquireQueryLock(h [32]byte) func() {
	c.locksMux.Lock()
	defer c.locksMux.Unlock()

	mux, ok := c.locks[h]
	if !ok {
		mux = &sync.Mutex{}
		c.locks[h] = mux
	}
	mux.Lock()

	return mux.Unlock
}

func (c *queryCache) intercept(ctx context.Context, in <-chan event.Event, hash [32]byte) <-chan event.Event {
	out := make(chan event.Event)

	var events []event.Event
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-in:
				if !ok {
					c.update(hash, events)
					return
				}

				select {
				case <-ctx.Done():
					return
				case out <- evt:
					events = append(events, evt)
				}
			}
		}
	}()

	return out
}

func (c *queryCache) update(hash [32]byte, events []event.Event) {
	c.cacheMux.Lock()
	c.cache[hash] = events
	c.cacheMux.Unlock()
}

// TODO(bounoable): Is this sufficient for avoiding collisions?
// Alternative: github.com/mitchellh/hashstructure
func hashQuery(q event.Query) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("%v", q)))
}

func eventStream(ctx context.Context, events []event.Event) (<-chan event.Event, <-chan error) {
	out := make(chan event.Event)
	errs := make(chan error)
	go func() {
		defer close(out)
		defer close(errs)
		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case out <- evt:
			}
		}
	}()
	return out, errs
}
