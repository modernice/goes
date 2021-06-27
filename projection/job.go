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
)

var (
	// ErrAggregateNotFound is returned when trying to extract an AggregateID
	// from a Job's Events and none of those Events belong to an aggregate with
	// that name.
	ErrAggregateNotFound = errors.New("aggregate not found in events")
)

// Job is a projection job. Jobs are typically created within Schedules and
// passed to subscribers of those Schedules.
type Job interface {
	// Context returns the Job's Context.
	Context() context.Context

	// Events fetches all Events that match the Job's Query and returns an Event
	// channel and a channel of asynchronous query errors.
	//
	//	var job Job
	//	str, errs, err := job.Events(job.Context())
	//	// handle err
	//	events, err := event.Drain(job.Context(), str, errs)
	//
	// Optional Queries may be provided as filters for the fetched Events. If
	// filters are provided, the returned Event channel will only receive Events
	// that match all provided Queries:
	//
	//	var job Job
	//	str, errs, err := job.Events(job.Context(), query.New(query.Name("foo")), query.New(...))
	//	// handle err
	//	events, err := event.Drain(job.Context(), str, errs)
	//
	// If you need the Events for a specific Projection, use EventsFor instead.
	Events(_ context.Context, filters ...event.Query) (<-chan event.Event, <-chan error, error)

	// EventsOf fetches all Events that belong to aggregates that have one of
	// aggregateNames and returns an Event channel and a channel of asynchronous
	// query errors.
	//
	//	var job Job
	//	str, errs, err := job.EventsOf(job.Context(), "foo", "bar", "baz")
	//	// handle err
	//	events, err := event.Drain(job.Context(), str, errs)
	EventsOf(_ context.Context, aggregateNames ...string) (<-chan event.Event, <-chan error, error)

	// EventsFor fetches all Events that are appropriate for the given
	// Projection and returns an Event channel and a channel of asynchronous
	// query errors. Which Events are queried depends on the Projection: If the
	// Projection implements guard (or embeds Guard), the Guard's Query is added
	// as a filter when querying Events. If the Projection implements progressor
	// (or embeds *Progressor), the progress time of the Projection is used to
	// only query Events that happened after that time.
	//
	//	var job Job
	//	var proj projection.Projection
	//	str, errs, err := job.EventsFor(job.Context(), proj)
	//	// handle err
	//	events, err := event.Drain(job.Context(), str, errs)
	EventsFor(context.Context, Projection) (<-chan event.Event, <-chan error, error)

	// Aggregates returns a channel of aggregate Tuples and a channel of
	// asynchronous query errors. It fetches Events, extracts the Tuples from
	// those Events and pushes them into the returned Tuple channel. Every
	// unique Tuple is guarenteed to be received exactly once, even if there are
	// muliple Events that belong to the same aggregate.
	//
	// If aggregateNames are provided, they are used to query only Events that
	// belong to one of the given aggregates.
	//
	//	var job Job
	//	str, errs, err := job.Aggregates(job.Context(), "foo", "bar", "baz")
	//	// handle err
	//	events, err := event.Drain(job.Context(), str, errs)
	Aggregates(_ context.Context, aggregateNames ...string) (<-chan aggregate.Tuple, <-chan error, error)

	// Aggregate returns the UUID of the first aggregate with the given
	// aggregateName that can be found in the Events of the Job, or
	// ErrAggregateNotFound if no Event belongs to an aggregate with that name.
	Aggregate(_ context.Context, aggregateName string) (uuid.UUID, error)

	// Apply applies the Job onto the Projection. A Job may be applied onto as
	// many Projections as needed.
	Apply(context.Context, Projection) error
}

// JobOption is a Job option.
type JobOption func(*job)

// WithFilter returns a JobOption that adds queries as filters to the Job.
// Fetched Events are matched against every Query and only returned in the
// result if they match all Queries.
func WithFilter(queries ...event.Query) JobOption {
	return func(j *job) {
		j.filter = append(j.filter, queries...)
	}
}

// WithReset returns a JobOption that resets Projections before applying Events
// onto them. Resetting a Projection is done by first resetting the progress of
// the Projection (if it implements progressor). Then, if the Projection has a
// Reset method, that method is called to allow for custom reset logic.
func WithReset() JobOption {
	return func(j *job) {
		j.reset = true
	}
}

// NewJob returns a new projection Job. The Job uses the provided Query to fetch
// the Events from the Store.
func NewJob(ctx context.Context, store event.Store, q event.Query, opts ...JobOption) Job {
	j := job{
		ctx:   ctx,
		query: q,
		cache: newQueryCache(store),
	}
	for _, opt := range opts {
		opt(&j)
	}
	if j.query == nil {
		j.query = query.New()
	}
	return &j
}

func (j *job) Context() context.Context {
	return j.ctx
}

func (j *job) Events(ctx context.Context, filter ...event.Query) (<-chan event.Event, <-chan error, error) {
	str, errs, err := j.runQuery(ctx, j.query)
	if err != nil {
		return nil, nil, fmt.Errorf("query Events: %w", err)
	}

	if filter = append(j.filter, filter...); len(filter) > 0 {
		str = event.Filter(str, filter...)
	}

	return str, errs, nil
}

func (j *job) EventsOf(ctx context.Context, aggregateName ...string) (<-chan event.Event, <-chan error, error) {
	return j.Events(ctx, query.New(query.AggregateName(aggregateName...)))
}

func (j *job) EventsFor(ctx context.Context, target Projection) (<-chan event.Event, <-chan error, error) {
	var filter []event.Query

	if guard, hasGuard := target.(guard); hasGuard {
		filter = append(filter, guard.ProjectionFilter()...)
	}

	if progressor, isProgressor := target.(progressor); isProgressor {
		if progress := progressor.Progress(); !progress.IsZero() {
			filter = append(filter, query.New(query.Time(time.After(progress))))
		}
	}

	return j.Events(ctx, filter...)
}

func (j *job) Aggregates(ctx context.Context, names ...string) (<-chan aggregate.Tuple, <-chan error, error) {
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

	out := make(chan aggregate.Tuple)
	found := make(map[aggregate.Tuple]bool)

	go func() {
		defer close(out)
		for evt := range events {
			tuple := aggregate.Tuple{
				Name: evt.AggregateName(),
				ID:   evt.AggregateID(),
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
	if err := aggregate.WalkTuples(ctx, func(t aggregate.Tuple) error {
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

func (j *job) Apply(ctx context.Context, proj Projection) error {
	if j.reset {
		if progressor, isProgressor := proj.(progressor); isProgressor {
			progressor.SetProgress(stdtime.Time{})
		}

		if resetter, isResetter := proj.(resetter); isResetter {
			resetter.Reset()
		}
	}

	str, errs, err := j.EventsFor(ctx, proj)
	if err != nil {
		return fmt.Errorf("fetch Events: %w", err)
	}

	events, err := event.Drain(ctx, str, errs)
	if err != nil {
		return fmt.Errorf("drain Events: %w", err)
	}

	if err := Apply(proj, events); err != nil {
		return fmt.Errorf("apply Events onto Projection: %w", err)
	}

	return nil
}

func (j *job) runQuery(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	return j.cache.ensure(ctx, q)
}

type job struct {
	ctx    context.Context
	query  event.Query
	filter []event.Query
	reset  bool
	cache  *queryCache
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
