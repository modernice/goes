package project

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/version"
)

type Context interface {
	context.Context

	AggregateName() string
	AggregateID() uuid.UUID

	Project(context.Context, Projection) error
}

type pcontext struct {
	context.Context
	Projector

	aggregateName string
	aggregateID   uuid.UUID
}

type Schedule interface {
	jobs(context.Context) (<-chan job, <-chan error, error)
}

type SubscribeOption func(*subscription)

type subscription struct {
	ctx    context.Context
	cancel context.CancelFunc

	stopTimeout time.Duration
	queryOpts   []query.Option
	query       query.Query

	proj     Projector
	schedule Schedule

	out        chan Context
	errs       chan error
	stop       chan struct{}
	handleDone chan struct{}
}

type ScheduleOption func(*scheduleConfig)

type scheduleConfig struct {
	filter []query.Option
}

type continously struct {
	filter query.Query
	bus    event.Bus
	events []string
}

type periodically struct {
	filter   query.Query
	store    event.Store
	interval time.Duration
	names    []string
}

type job struct {
	aggregateName string
	aggregateID   uuid.UUID
}

func Continuously(bus event.Bus, events []string, opts ...ScheduleOption) Schedule {
	cfg := newScheduleConfig(opts...)
	c := &continously{
		filter: query.New(cfg.filter...),
		bus:    bus,
		events: events,
	}
	return c
}

func newScheduleConfig(opts ...ScheduleOption) scheduleConfig {
	var cfg scheduleConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func Periodically(store event.Store, d time.Duration, aggregates []string, opts ...ScheduleOption) Schedule {
	cfg := newScheduleConfig(opts...)
	return &periodically{
		filter:   query.New(cfg.filter...),
		store:    store,
		interval: d,
		names:    aggregates,
	}
}

func StopTimeout(d time.Duration) SubscribeOption {
	return func(s *subscription) {
		s.stopTimeout = d
	}
}

func FilterEvents(opts ...query.Option) ScheduleOption {
	return func(cfg *scheduleConfig) {
		cfg.filter = append(cfg.filter, opts...)
	}
}

func Subscribe(
	ctx context.Context,
	proj Projector,
	s Schedule,
	opts ...SubscribeOption,
) (<-chan Context, <-chan error, error) {
	sub := newSubscription(ctx, proj, s, opts...)

	jobs, errs, err := s.jobs(ctx)
	if err != nil {
		return nil, nil, err
	}

	go sub.handleJobs(jobs)
	go sub.handleErrors(errs)
	go sub.handleCancel()

	return sub.out, sub.errs, nil
}

func (sub *subscription) handleJobs(jobs <-chan job) {
	defer close(sub.handleDone)
	defer close(sub.out)
	for job := range jobs {
		sub.handleJob(job)
	}
}

func (sub *subscription) handleJob(j job) {
	select {
	case <-sub.stop:
	case sub.out <- newContext(
		sub.ctx,
		sub.proj,
		j.aggregateName,
		j.aggregateID,
	):
	}
}

func (sub *subscription) handleErrors(errs <-chan error) {
	defer close(sub.errs)
	for err := range errs {
		sub.errs <- err
		// TODO: timeout + buffer
	}
}

func (sub *subscription) handleCancel() {
	defer close(sub.stop)

	<-sub.ctx.Done()

	var timeout <-chan time.Time
	if sub.stopTimeout > 0 {
		timer := time.NewTimer(sub.stopTimeout)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-sub.handleDone:
	case <-timeout:
	}
}

func (c *continously) jobs(ctx context.Context) (<-chan job, <-chan error, error) {
	events, _, err := c.bus.Subscribe(ctx, c.events...)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to %v events: %w", c.events, err)
	}

	jobs := make(chan job)
	errs := make(chan error)
	go c.handleEvents(events, jobs, errs)

	return jobs, errs, err
}

func (c *continously) handleEvents(
	events <-chan event.Event,
	jobs chan<- job,
	errs chan<- error,
) {
	defer close(jobs)
	defer close(errs)
	for evt := range events {
		if shouldDiscard(evt, c.filter) {
			continue
		}

		jobs <- job{
			aggregateName: evt.AggregateName(),
			aggregateID:   evt.AggregateID(),
		}
	}
}

func (p *periodically) jobs(ctx context.Context) (<-chan job, <-chan error, error) {
	ticker := time.NewTicker(p.interval)
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
	jobs := make(chan job)
	errs := make(chan error)
	go p.handleTicks(ctx, ticker.C, jobs, errs)
	return jobs, errs, nil
}

func (p *periodically) handleTicks(
	ctx context.Context,
	ticker <-chan time.Time,
	jobs chan<- job,
	errs chan<- error,
) {
	defer close(jobs)
	defer close(errs)
L:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			str, serrs, err := p.store.Query(ctx, query.New(
				query.AggregateName(p.names...),
				query.AggregateVersion(version.Exact(0)),
			))
			if err != nil {
				errs <- fmt.Errorf("query base events: %w", err)
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case err, ok := <-serrs:
					if !ok {
						serrs = nil
						break
					}
					errs <- fmt.Errorf("event stream: %w", err)
				case evt, ok := <-str:
					if !ok {
						continue L
					}

					if shouldDiscard(evt, p.filter) {
						continue
					}

					select {
					case <-ctx.Done():
						return
					case jobs <- job{
						aggregateName: evt.AggregateName(),
						aggregateID:   evt.AggregateID(),
					}:
					}
				}
			}
		}
	}
}

func (j *pcontext) AggregateName() string {
	return j.aggregateName
}

func (j *pcontext) AggregateID() uuid.UUID {
	return j.aggregateID
}

func newSubscription(ctx context.Context, proj Projector, s Schedule, opts ...SubscribeOption) *subscription {
	ctx, cancel := context.WithCancel(ctx)
	sub := subscription{
		ctx:        ctx,
		cancel:     cancel,
		proj:       proj,
		schedule:   s,
		out:        make(chan Context),
		errs:       make(chan error),
		stop:       make(chan struct{}),
		handleDone: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(&sub)
	}
	sub.query = query.New(sub.queryOpts...)
	return &sub
}

func newContext(ctx context.Context, proj Projector, name string, id uuid.UUID) *pcontext {
	return &pcontext{
		Context:       ctx,
		Projector:     proj,
		aggregateName: name,
		aggregateID:   id,
	}
}

func shouldDiscard(evt event.Event, q query.Query) bool {
	if evt.AggregateName() == "" || evt.AggregateID() == uuid.Nil {
		return true
	}
	return !query.Test(q, evt)
}
