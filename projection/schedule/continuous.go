package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/projection"
)

// defaultDebounceBarrier defines when DebounceCap doubles the wait instead of
// using DefaultDebounceCap.
const defaultDebounceBarrier = 2500 * time.Millisecond

// DefaultDebounceCap is used when Debounce is <=2.5s and no cap is set.
// Longer durations default to double the debounce.
var DefaultDebounceCap = 5 * time.Second

// Continuous triggers a job for every matching published event.
type Continuous struct {
	*schedule

	bus                    event.Bus
	debounce               time.Duration
	debounceCap            time.Duration
	debounceCapManuallySet bool
}

// ContinuousOption configures a [Continuous] schedule.
type ContinuousOption func(*Continuous)

// Debounce merges events within d into a single job.
func Debounce(d time.Duration) ContinuousOption {
	return func(c *Continuous) {
		c.debounce = d
	}
}

// DebounceCap sets the maximum wait before a debounced job runs.
func DebounceCap(cap time.Duration) ContinuousOption {
	return func(c *Continuous) {
		c.debounceCap = cap
		c.debounceCapManuallySet = true
	}
}

// Continuously builds a Continuous schedule that listens for eventNames.
func Continuously(bus event.Bus, store event.Store, eventNames []string, opts ...ContinuousOption) *Continuous {
	c := Continuous{
		schedule:    newSchedule(store, eventNames),
		bus:         bus,
		debounceCap: DefaultDebounceCap,
	}
	for _, opt := range opts {
		opt(&c)
	}

	return &c
}

// Subscribe wires apply to incoming jobs and returns a channel of async errors.
func (schedule *Continuous) Subscribe(ctx context.Context, apply func(projection.Job) error, opts ...projection.SubscribeOption) (<-chan error, error) {
	cfg := projection.NewSubscription(opts...)

	events, errs, err := schedule.bus.Subscribe(ctx, schedule.eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v events: %w", schedule.eventNames, err)
	}

	out := make(chan error)
	jobs := make(chan projection.Job)
	triggers := schedule.newTriggers()
	done := make(chan struct{})

	go func() {
		<-done
		schedule.removeTriggers(triggers)
	}()

	if cfg.Startup != nil {
		if err := schedule.applyStartupJob(ctx, cfg, jobs, apply); err != nil {
			return nil, fmt.Errorf("startup: %w", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go schedule.handleEvents(ctx, cfg, events, errs, jobs, out, &wg)
	go schedule.handleTriggers(ctx, cfg, triggers, jobs, out, &wg)
	go schedule.applyJobs(ctx, apply, jobs, out, done)

	go func() {
		wg.Wait()
		close(jobs)
	}()

	return out, nil
}

func (schedule *Continuous) handleEvents(
	ctx context.Context,
	sub projection.Subscription,
	events <-chan event.Event,
	errs <-chan error,
	jobs chan<- projection.Job,
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
	var buf []event.Event
	var debounce, debounceCap *time.Timer
	var jobCreated bool

	clearDebounce := func() {
		mux.Lock()
		defer mux.Unlock()

		jobCreated = false

		if debounce != nil {
			debounce.Stop()
			debounce = nil
		}

		if debounceCap != nil {
			debounceCap.Stop()
			debounceCap = nil
		}
	}

	defer clearDebounce()

	createJob := func() {
		defer clearDebounce()

		mux.Lock()
		defer mux.Unlock()

		if jobCreated {
			return
		}

		events := make([]event.Event, len(buf))
		copy(events, buf)

		job := schedule.newJob(
			ctx,
			sub,
			eventstore.New(events...),
			query.New(query.SortBy(event.SortTime, event.SortAsc)),
		)

		select {
		case <-ctx.Done():
		case jobs <- job:
		}

		buf = buf[:0]
		jobCreated = true
	}

	addEvent := func(evt event.Event) {
		clearDebounce()

		buf = append(buf, evt)

		if schedule.debounce <= 0 {
			createJob()
			return
		}

		mux.Lock()
		defer mux.Unlock()

		debounce = time.AfterFunc(schedule.debounce, createJob)

		if cap := schedule.computeDebounceCap(); cap > 0 {
			debounceCap = time.AfterFunc(cap, createJob)
		}
	}

	streams.ForEach(ctx, addEvent, fail, events, errs)
}

func (s *Continuous) computeDebounceCap() time.Duration {
	if s.debounceCap <= 0 {
		return 0
	}

	if s.debounceCapManuallySet {
		return s.debounceCap
	}

	if s.debounce <= defaultDebounceBarrier {
		return s.debounceCap
	}

	return s.debounce * 2
}
