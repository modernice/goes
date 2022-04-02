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
	// ErrAggregateNotFound is returned when trying to extract an aggregateID
	// from a Job's events and none of those Events belong to an aggregate with
	// that name.
	ErrAggregateNotFound = errors.New("aggregate not found in events")
)

// Job is a projection job. Jobs are typically created within Schedules and
// passed to subscribers of those Schedules.
type Job interface {
	context.Context

	// Events fetches all events that match the Job's Query and returns an Event
	// channel and a channel of asynchronous query errors.
	//
	//	var job Job
	//	str, errs, err := job.Events(job)
	//	// handle err
	//	events, err := streams.Drain(job, str, errs)
	//
	// Optional Queries may be provided as filters for the fetched events. If
	// filters are provided, the returned event channel will only receive Events
	// that match all provided Queries:
	//
	//	var job Job
	//	str, errs, err := job.Events(job, query.New(query.Name("foo")), query.New(...))
	//	// handle err
	//	events, err := streams.Drain(job, str, errs)
	//
	// If you need the events for a specific projection, use EventsFor instead.
	Events(_ context.Context, filters ...event.Query) (<-chan event.Event, <-chan error, error)

	// EventsOf fetches all events that belong to aggregates that have one of
	// aggregateNames and returns an event channel and a channel of asynchronous
	// query errors.
	//
	//	var job Job
	//	str, errs, err := job.EventsOf(job, "foo", "bar", "baz")
	//	// handle err
	//	events, err := streams.Drain(job, str, errs)
	EventsOf(_ context.Context, aggregateNames ...string) (<-chan event.Event, <-chan error, error)

	// EventsFor fetches all events that are appropriate for the given
	// Projection and returns an event channel and a channel of asynchronous
	// query errors. Which events are queried depends on the projection: If the
	// Projection implements guard (or embeds Guard), the Guard's Query is added
	// as a filter when querying events. If the projection implements progressor
	// (or embeds *Progressor), the progress time of the projection is used to
	// only query events that happened after that time.
	//
	//	var job Job
	//	var proj projection.Projection
	//	str, errs, err := job.EventsFor(job, proj)
	//	// handle err
	//	events, err := streams.Drain(job, str, errs)
	EventsFor(context.Context, EventApplier[any]) (<-chan event.Event, <-chan error, error)

	// Aggregates returns a channel of aggregate Tuples and a channel of
	// asynchronous query errors. It fetches events, extracts the Tuples from
	// those events and pushes them into the returned Tuple channel. Every
	// unique Tuple is guarenteed to be received exactly once, even if there are
	// muliple events that belong to the same aggregate.
	//
	// If aggregateNames are provided, they are used to query only events that
	// belong to one of the given aggregates.
	//
	//	var job Job
	//	str, errs, err := job.Aggregates(job, "foo", "bar", "baz")
	//	// handle err
	//	events, err := streams.Drain(job, str, errs)
	Aggregates(_ context.Context, aggregateNames ...string) (<-chan aggregate.Ref, <-chan error, error)

	// Aggregate returns the UUID of the first aggregate with the given
	// aggregateName that can be found in the events of the Job, or
	// ErrAggregateNotFound if no event belongs to an aggregate with that name.
	Aggregate(_ context.Context, aggregateName string) (uuid.UUID, error)

	// Apply applies the Job onto the projection. A Job may be applied onto as
	// many projections as needed.
	Apply(context.Context, EventApplier[any], ...ApplyOption) error
}

// JobOption is a Job option.
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

	// Store that is used for projections that implement HistoryDependent, if
	// their RequiresFullHistory method returns true.
	historyStore event.Store
}

// WithFilter returns a JobOption that adds queries as filters to the Job.
// Fetched events are matched against every Query and only returned in the
// result if they match all Queries.
func WithFilter(queries ...event.Query) JobOption {
	return func(j *job) {
		j.filter = append(j.filter, queries...)
	}
}

// WithReset returns a JobOption that resets projections before applying events
// onto them. Resetting a projection is done by first resetting the progress of
// the projection (if it implements ProgressAware). Then, if the Projection has a
// Reset method, that method is called to allow for custom reset logic.
func WithReset() JobOption {
	return func(j *job) {
		j.reset = true
	}
}

// WithAggregateQuery returns a JobOption that specifies the event query that is
// used for the `Aggregates()` and `Aggregate()` methods of a job. If this
// option is not provided, the main query of the job is used instead.
func WithAggregateQuery(q event.Query) JobOption {
	return func(j *job) {
		j.aggregateQuery = q
	}
}

// WithBeforeEvent returns a JobOption that adds the given functions as
// "before"-interceptors to the event streams returned by a job's `EventsFor()`
// and `Apply()` methods. For each received event of a stream, all provided
// functions are called in order, and the returned events are inserted into the
// stream before the intercepted event.
func WithBeforeEvent(fns ...func(context.Context, event.Event) ([]event.Event, error)) JobOption {
	return func(j *job) {
		j.beforeEvent = append(j.beforeEvent, fns...)
	}
}

// WithHistoryStore returns a JobOption that provides the projection job with an
// event store that is used to query events for projections that require the
// full event history to build the projection state.
func WithHistoryStore(store event.Store) JobOption {
	return func(j *job) {
		j.historyStore = store
	}
}

// NewJob returns a new projection Job. The Job uses the provided Query to fetch
// the events from the Store.
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

func (j *job) EventsOf(ctx context.Context, aggregateName ...string) (<-chan event.Event, <-chan error, error) {
	return j.Events(ctx, query.New(query.AggregateName(aggregateName...)))
}

func (j *job) EventsFor(ctx context.Context, target EventApplier[any]) (<-chan event.Event, <-chan error, error) {
	q := j.query

	if progressor, isProgressor := target.(ProgressAware); isProgressor {
		progressTime, _ := progressor.Progress()
		if !progressTime.IsZero() {
			// Why subtract a nanosecond and return possibly already applied
			// events? Because multiple events can have the same time, and we
			// want to ensure that we don't accidentally exclude events that
			// haven't been applied yet. The Apply and ApplyStream functions
			// ensure that an event is not applied twice to a projection.
			q = query.Merge(q, query.New(query.Time(
				time.After(progressTime.Add(-stdtime.Nanosecond))),
			))
		}
	}

	if hp, ok := target.(HistoryDependent); ok && hp.RequiresFullHistory() {
		if j.historyStore == nil {
			return nil, nil, fmt.Errorf("projection requires full history, but job has no history event store")
		}

		str, errs, err := j.queryHistory(ctx, j.query)
		if err != nil {
			return str, errs, fmt.Errorf("query history: %w", err)
		}

		return event.Filter(str), errs, nil
	}

	return j.queryEvents(ctx, q)
}

// TODO(bounoable): Actually run the query only once.
func (j *job) queryHistory(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	return j.historyStore.Query(ctx, j.query)
}

func (j *job) Aggregates(ctx context.Context, names ...string) (<-chan aggregate.Ref, <-chan error, error) {
	var (
		events <-chan event.Event
		errs   <-chan error
		err    error
	)

	if len(names) == 0 {
		events, errs, err = j.Events(ctx)
	} else {
		events, errs, err = j.EventsOf(ctx, names...)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("fetch Events: %w", err)
	}

	out := make(chan aggregate.Ref)
	found := make(map[aggregate.Ref]bool)

	go func() {
		defer close(out)
		for evt := range events {
			id, name, _ := evt.Aggregate()
			tuple := aggregate.Ref{
				Name: name,
				ID:   id,
			}

			if found[tuple] {
				continue
			}
			found[tuple] = true

			select {
			case <-ctx.Done():
				return
			case out <- tuple:
			}
		}
	}()

	return out, errs, nil
}

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

func (j *job) Apply(ctx context.Context, proj EventApplier[any], opts ...ApplyOption) error {
	if j.reset {
		if progressor, isProgressor := proj.(ProgressAware); isProgressor {
			progressor.SetProgress(stdtime.Time{})
		}

		if resetter, isResetter := proj.(Resetter); isResetter {
			resetter.Reset()
		}
	}

	events, errs, err := j.EventsFor(ctx, proj)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		ApplyStream(proj, events, opts...)
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
