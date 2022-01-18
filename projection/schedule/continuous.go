package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
)

// Continuous is a projection Schedule that creates projection Jobs on every
// specified published Event:
//
//	var bus event.Bus
//	var store event.Store
//	var proj projection.Projection
//	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
//	errs, err := s.Subscribe(context.TODO(), func(job projection.Job) error {
//		return job.Apply(job, proj)
//	})
type Continuous[E any] struct {
	*schedule[E]

	bus      event.Bus[E]
	debounce time.Duration
}

// ContinuousOption is an option for the Continuous schedule.
type ContinuousOption[E any] func(*Continuous[E])

// Debounce returns a ContinuousOption that debounces projection Jobs by the
// given Duration. When multiple Events are published within the given Duration,
// only 1 projection Job for all Events will be created instead of 1 Job per
// Event.
//
//	var bus event.Bus
//	var store event.Store
//	var proj projection.Projection
//	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"}, schedule.Debounce(time.Second))
//	errs, err := s.Subscribe(context.TODO(), func(job projection.Job) error {
//		return job.Apply(job, proj)
//	})
//
//	err := bus.Publish(
//		context.TODO(),
//		event.New("foo", ...),
//		event.New("bar", ...),
//		event.New("baz", ...),
//	)
func Debounce[E any](d time.Duration) ContinuousOption[E] {
	return func(c *Continuous[E]) {
		c.debounce = d
	}
}

// Continuously returns a Continuous schedule that, when subscribed to,
// subscribes to Events with the given eventNames to create projection Jobs
// for those Events.
//
// Debounce Events
//
// It may be desirable to debounce the creation of projection Jobs to avoid
// creating a Job on every Event if Events are published within a short
// interval:
//
//	var bus event.Bus
//	var store event.Store
//	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"}, schedule.Debounce(time.Second))
func Continuously[E any](bus event.Bus[E], store event.Store[E], eventNames []string, opts ...ContinuousOption[E]) *Continuous[E] {
	c := Continuous[E]{
		schedule: newSchedule(store, eventNames),
		bus:      bus,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

// Subscribe subscribes to the schedule and returns a channel of asynchronous
// projection errors, or a single error if subscribing failed. When ctx is
// canceled, the subscription is canceled and the returned error channel closed.
//
// When a projection Job is created, the apply function is called with that Job.
// Use Job.Apply to apply the Job's Events onto a given Projection:
//
//	var proj projection.Projection
//	var s *schedule.Continuous
//	s.Subscribe(context.TODO(), func(job projection.Job) error {
//		return job.Apply(job, proj)
//	})
//
// A Job provides helper functions to extract data from the Job's Events. Query
// results are cached within a Job, so it is safe to call helper functions
// multiple times; the Job will figure out if it needs to actually perform the
// query or if it can return the cached result.
//
//	s.Subscribe(context.TODO(), func(job projection.Job) error {
//		events, errs, err := job.Events(job) // fetch all events of the Job
//		events, errs, err := job.Events(job, query.New(...)) // fetch events with filter
//		events, errs, err := job.EventsOf(job, "foo", "bar") // fetch events that belong to specific aggregates
//		events, errs, err := job.EventsFor(job, proj) // fetch events that would be applied onto proj
//		tuples, errs, err := job.Aggregates(job) // extract aggregates from events
//		tuples, errs, err := job.Aggregates(job, "foo", "bar") // extract specific aggregates from events
//		id, err := job.Aggregate(job, "foo") // extract UUID of first aggregate with given name
//	})
//
// When the schedule is triggered by calling schedule.Trigger, a projection Job
// will be created and passed to apply.
func (schedule *Continuous[E]) Subscribe(ctx context.Context, apply func(projection.Job[E]) error) (<-chan error, error) {
	events, errs, err := schedule.bus.Subscribe(ctx, schedule.eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to events: %w (Events=%v)", err, schedule.eventNames)
	}

	out := make(chan error)
	jobs := make(chan projection.Job[E])
	triggers := schedule.newTriggers()
	done := make(chan struct{})

	go func() {
		<-done
		schedule.removeTriggers(triggers)
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		wg.Wait()
		close(jobs)
	}()

	go schedule.handleEvents(ctx, events, errs, jobs, out, &wg)
	go schedule.handleTriggers(ctx, triggers, jobs, out, &wg)
	go schedule.applyJobs(ctx, apply, jobs, out, done)

	return out, nil
}

func (schedule *Continuous[E]) handleEvents(
	ctx context.Context,
	events <-chan event.EventOf[E],
	errs <-chan error,
	jobs chan<- projection.Job[E],
	out chan<- error,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	fail := func(err error) {
		select {
		case <-ctx.Done():
		case out <- err:
		}
	}

	var mux sync.Mutex
	var buf []event.EventOf[E]
	var debounce *time.Timer

	defer func() {
		mux.Lock()
		defer mux.Unlock()
		if debounce != nil {
			debounce.Stop()
		}
	}()

	createJob := func() {
		mux.Lock()
		defer mux.Unlock()

		events := make([]event.EventOf[E], len(buf))
		copy(events, buf)

		job := projection.NewJob[E](
			ctx,
			eventstore.New(events...),
			query.New(query.SortBy(event.SortTime, event.SortAsc)),
			projection.WithHistoryStore[E](schedule.store),
		)

		select {
		case <-ctx.Done():
		case jobs <- job:
		}

		buf = buf[:0]
		debounce = nil
	}

	addEvent := func(evt event.EventOf[E]) {
		mux.Lock()

		if debounce != nil {
			debounce.Stop()
			debounce = nil
		}

		buf = append(buf, evt)

		if schedule.debounce <= 0 {
			mux.Unlock()
			createJob()
			return
		}

		defer mux.Unlock()

		if schedule.debounce > 0 {
			debounce = time.AfterFunc(schedule.debounce, createJob)
		}
	}

	event.ForEach(ctx, addEvent, fail, events, errs)
}
