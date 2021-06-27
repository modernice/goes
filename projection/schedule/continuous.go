package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
)

type Continuous struct {
	bus        event.Bus
	store      event.Store
	eventNames []string
	debounce   time.Duration

	triggersMux sync.RWMutex
	triggers    []chan projection.Trigger
}

type ContinuousOption func(*Continuous)

func Debounce(d time.Duration) ContinuousOption {
	return func(c *Continuous) {
		c.debounce = d
	}
}

func Continuously(bus event.Bus, store event.Store, eventNames []string, opts ...ContinuousOption) *Continuous {
	c := Continuous{
		bus:        bus,
		store:      store,
		eventNames: eventNames,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

func (schedule *Continuous) Subscribe(ctx context.Context, apply func(projection.Job) error) (<-chan error, error) {
	events, errs, err := schedule.bus.Subscribe(ctx, schedule.eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v Events: %w", schedule.eventNames, err)
	}

	out := make(chan error)
	jobs := make(chan projection.Job)
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

func (schedule *Continuous) Trigger(ctx context.Context, opts ...projection.TriggerOption) error {
	schedule.triggersMux.RLock()
	triggers := make([]chan projection.Trigger, len(schedule.triggers))
	copy(triggers, schedule.triggers)
	schedule.triggersMux.RUnlock()

	for _, triggers := range triggers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case triggers <- schedule.newTrigger(opts...):
		}
	}

	return nil
}

func (schedule *Continuous) newTriggers() <-chan projection.Trigger {
	triggers := make(chan projection.Trigger)

	schedule.triggersMux.Lock()
	schedule.triggers = append(schedule.triggers, triggers)
	schedule.triggersMux.Unlock()

	return triggers
}

func (schedule *Continuous) newTrigger(opts ...projection.TriggerOption) projection.Trigger {
	t := projection.NewTrigger(opts...)
	if t.Query == nil {
		t.Query = query.New(query.Name(schedule.eventNames...), query.SortBy(event.SortTime, event.SortAsc))
	}
	return t
}

func (schedule *Continuous) removeTriggers(triggers <-chan projection.Trigger) {
	schedule.triggersMux.Lock()
	defer schedule.triggersMux.Unlock()
	for i, striggers := range schedule.triggers {
		if striggers == triggers {
			schedule.triggers = append(schedule.triggers[:i], schedule.triggers[i+1:]...)
			return
		}
	}
}

func (schedule *Continuous) handleEvents(
	ctx context.Context,
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
	var debounce *time.Timer

	createJob := func() {
		mux.Lock()
		defer mux.Unlock()

		events := make([]event.Event, len(buf))
		copy(events, buf)

		job := projection.NewJob(ctx, memstore.New(events...), query.New(
			query.SortBy(event.SortTime, event.SortAsc),
		))

		select {
		case <-ctx.Done():
		case jobs <- job:
		}

		buf = buf[:0]
		debounce = nil
	}

	addEvent := func(evt event.Event) {
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

	event.ForEvery(ctx, addEvent, fail, events, errs)
}

func (schedule *Continuous) handleTriggers(
	ctx context.Context,
	triggers <-chan projection.Trigger,
	jobs chan<- projection.Job,
	out chan<- error,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case trigger := <-triggers:
			job := projection.NewJob(ctx, schedule.store, trigger.Query, projection.WithFilter(trigger.Filter...))
			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}
}

func (schedule *Continuous) applyJobs(
	ctx context.Context,
	apply func(projection.Job) error,
	jobs <-chan projection.Job,
	out chan<- error,
	done chan struct{},
) {
	defer close(done)
	defer close(out)
	for job := range jobs {
		if err := apply(job); err != nil {
			select {
			case <-ctx.Done():
				return
			case out <- fmt.Errorf("apply Job: %w", err):
			}
		}
	}
}
