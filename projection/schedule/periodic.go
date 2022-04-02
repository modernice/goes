package schedule

import (
	"context"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
)

// Periodic is a projection schedule that creates projection Jobs in a defined
// interval.
type Periodic struct {
	*schedule

	interval time.Duration
}

// Periodically returns a Periodic schedule that, when subscribed to, creates a
// projection Job every interval Duration and passes that Job to every
// subscriber of the schedule.
func Periodically(store event.Store, interval time.Duration, eventNames []string) *Periodic {
	return &Periodic{
		schedule: newSchedule(store, eventNames),
		interval: interval,
	}
}

// Subscribe subscribes to the schedule and returns a channel of asynchronous
// projection errors, or a single error if subscribing failed. When ctx is
// canceled, the subscription is canceled and the returned error channel closed.
//
// When a projection Job is created, the apply function is called with that Job.
// Use Job.Apply to apply the Job's events onto a given projection:
//
//	var proj projection.Projection
//	var s *schedule.Periodic
//	s.Subscribe(context.TODO(), func(job projection.Job) error {
//		return job.Apply(job, proj)
//	})
//
// A Job provides helper functions to extract data from the Job's events. Query
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
func (schedule *Periodic) Subscribe(ctx context.Context, apply func(projection.Job) error, opts ...projection.SubscribeOption) (<-chan error, error) {
	cfg := projection.NewSubscription(opts...)

	ticker := time.NewTicker(schedule.interval)

	out := make(chan error)
	jobs := make(chan projection.Job)
	triggers := schedule.newTriggers()
	done := make(chan struct{})

	go func() {
		<-done
		schedule.removeTriggers(triggers)
		ticker.Stop()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go schedule.handleTicker(ctx, cfg, ticker, jobs, out, &wg)
	go schedule.handleTriggers(ctx, triggers, jobs, out, &wg)
	go schedule.applyJobs(ctx, apply, jobs, out, done)

	if cfg.Startup != nil {
		wg.Add(1)
		go schedule.triggerStartupJob(ctx, cfg, jobs, &wg)
	}

	go func() {
		wg.Wait()
		close(jobs)
	}()

	return out, nil
}

func (schedule *Periodic) handleTicker(
	ctx context.Context,
	sub projection.Subscription,
	ticker *time.Ticker,
	jobs chan<- projection.Job,
	out chan<- error,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job := schedule.newJob(
				ctx,
				sub,
				schedule.store,
				query.New(
					query.Name(schedule.eventNames...),
					query.SortByAggregate(),
				),
				projection.WithHistoryStore(schedule.store),
			)

			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}
}
