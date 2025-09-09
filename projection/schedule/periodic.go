package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
)

// Periodic triggers projection jobs on a fixed interval.
type Periodic struct {
	*schedule

	interval time.Duration
}

// Periodically builds a Periodic schedule that emits a job every interval.
func Periodically(store event.Store, interval time.Duration, eventNames []string) *Periodic {
	return &Periodic{
		schedule: newSchedule(store, eventNames),
		interval: interval,
	}
}

// Subscribe wires apply to interval jobs and returns a channel of async errors.
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

	if cfg.Startup != nil {
		if err := schedule.applyStartupJob(ctx, cfg, jobs, apply); err != nil {
			return nil, fmt.Errorf("startup: %w", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go schedule.handleTicker(ctx, cfg, ticker, jobs, out, &wg)
	go schedule.handleTriggers(ctx, cfg, triggers, jobs, out, &wg)
	go schedule.applyJobs(ctx, apply, jobs, out, done)

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
					query.SortByTime(),
				),
			)

			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}
}
