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
	str, errs, err := j.runQuery(ctx, j.query)
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	if filter = append(j.filter, filter...); len(filter) > 0 {
		str = event.Filter(str, filter...)
	}

	return str, errs, nil
}

func (j *job) EventsOf(ctx context.Context, aggregateName ...string) (<-chan event.Event, <-chan error, error) {
	return j.Events(ctx, query.New(query.AggregateName(aggregateName...)))
}

func (j *job) EventsFor(ctx context.Context, target EventApplier[any]) (<-chan event.Event, <-chan error, error) {
	var filter []event.Query

	if progressor, isProgressor := target.(ProgressAware); isProgressor {
		progressTime, _ := progressor.Progress()
		if !progressTime.IsZero() {
			filter = append(filter, query.New(query.Time(time.After(progressTime))))
		}
	}

	if hp, ok := target.(HistoryDependent); ok && hp.RequiresFullHistory() {
		if j.historyStore == nil {
			return nil, nil, fmt.Errorf("projection requires full history, but job has no history event store")
		}

		str, errs, err := j.queryOnce(ctx, j.query)
		if err != nil {
			return str, errs, fmt.Errorf("query history: %w", err)
		}

		return event.Filter(str, filter...), errs, nil
	}

	return j.Events(ctx, filter...)
}

// TODO(bounoable): Actually run the query only once.
func (j *job) queryOnce(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
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
	opts = append([]ApplyOption{IgnoreProgress()}, opts...)

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
	return j.cache.ensure(ctx, q)
}

type job struct {
	context.Context

	query  event.Query
	filter []event.Query
	reset  bool
	cache  *queryCache

	// Store that is used for projections that implement HistoryDependent, if
	// their RequiresFullHistory method returns true.
	historyStore event.Store
}

type queryCache struct {
	sync.Mutex

	store event.Store
	cache map[[32]byte][]event.Event
}

func newQueryCache(store event.Store) *queryCache {
	return &queryCache{
		store: store,
		cache: make(map[[32]byte][]event.Event),
	}
}

func (c *queryCache) ensure(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	h := hashQuery(q)

	var events []event.Event

	c.Lock()
	if cached, ok := c.cache[h]; ok {
		events = make([]event.Event, len(cached))
		copy(events, cached)
	}
	c.Unlock()

	if len(events) > 0 {
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
		return out, errs, nil
	}

	str, errs, err := c.store.Query(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("query Events: %w", err)
	}

	return c.intercept(ctx, str, h), errs, nil
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
	c.Lock()
	c.cache[hash] = events
	c.Unlock()
}

func hashQuery(q event.Query) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("%v", q)))
}
