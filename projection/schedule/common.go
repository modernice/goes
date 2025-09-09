package schedule

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
)

type schedule struct {
	store      event.Store
	eventNames []string

	triggersMux sync.RWMutex
	triggers    []chan projection.Trigger
}

func newSchedule(store event.Store, eventNames []string) *schedule {
	return &schedule{
		store:      store,
		eventNames: eventNames,
	}
}

// Trigger creates a job and delivers it to subscribers. The job's query and filters can be adjusted with TriggerOptions. Only ctx.Err() may be returned.
func (schedule *schedule) Trigger(ctx context.Context, opts ...projection.TriggerOption) error {
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

func (schedule *schedule) newTriggers() <-chan projection.Trigger {
	schedule.triggersMux.Lock()
	defer schedule.triggersMux.Unlock()

	triggers := make(chan projection.Trigger)
	schedule.triggers = append(schedule.triggers, triggers)

	return triggers
}

func (schedule *schedule) newTrigger(opts ...projection.TriggerOption) projection.Trigger {
	t := projection.NewTrigger(opts...)
	if t.Query == nil {
		t.Query = query.New(query.Name(schedule.eventNames...), query.SortByTime())
	}
	return t
}

func (schedule *schedule) removeTriggers(triggers <-chan projection.Trigger) {
	schedule.triggersMux.Lock()
	defer schedule.triggersMux.Unlock()
	for i, striggers := range schedule.triggers {
		if striggers == triggers {
			schedule.triggers = append(schedule.triggers[:i], schedule.triggers[i+1:]...)
			return
		}
	}
}

func (schedule *schedule) handleTriggers(
	ctx context.Context,
	sub projection.Subscription,
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
			q := trigger.Query
			if q == nil {
				q = query.New(query.Name(schedule.eventNames...), query.SortByTime())
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- schedule.newJob(ctx, sub, schedule.store, q, trigger.JobOptions()...):
			}
		}
	}
}

func (schedule *schedule) applyJobs(
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
			case out <- fmt.Errorf("apply job: %w", err):
			}
		}
	}
}

func (schedule *schedule) applyStartupJob(
	ctx context.Context,
	sub projection.Subscription,
	jobs chan<- projection.Job,
	apply func(projection.Job) error,
) error {
	if sub.Startup == nil {
		return nil
	}

	q := sub.Startup.Query
	if q == nil {
		q = query.New(query.Name(schedule.eventNames...), query.SortByTime())
	}

	return apply(schedule.newJob(
		ctx,
		sub,
		schedule.store,
		q,
		sub.Startup.JobOptions()...,
	))
}

func (schedule *schedule) newJob(ctx context.Context, sub projection.Subscription, store event.Store, q event.Query, opts ...projection.JobOption) projection.Job {
	return projection.NewJob(ctx, store, q, append([]projection.JobOption{
		projection.WithBeforeEvent(sub.BeforeEvent...),
	}, opts...)...)
}
