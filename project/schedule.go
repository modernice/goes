package project

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

// A Schedule is used to subscribe a Projector to an Event stream.
type Schedule interface {
	// Subscribe subscribes to the Schedule and returns a channel of Jobs and errors.
	//
	// Basic usage
	//
	// Subscribe to a Schedule and provide an applyFunc that applies the Job
	// onto as many projections as needed:
	//
	//	var s project.Schedule
	//	errs, err := s.Subscribe(context.TODO(), func(j project.Job) error {
	//		foo := newFoo()
	//		if err := j.Apply(j.Context(), foo); err != nil {
	//			return fmt.Errorf("apply Job: %w", err)
	//		}
	//		return nil
	//	})
	//	// handle err & errs
	//
	// Filter Events
	//
	// An Event Query can be provided to filter which Events are applied on a projection:
	//
	//	var s project.Schedule
	//	q := query.New(query.AggregateName("foobar")) // only apply Events that belong to a "foobar" Aggregate
	//	errs, err := s.Subscribe(context.TODO(), func(j project.Job) error { ... }, project.Filter(q))
	//	// handle err & errs
	Subscribe(context.Context, func(Job) error, ...SubscribeOption) (<-chan error, error)

	// Trigger triggers the Schedule to create a Job and pass it to the
	// applyFuncs that have been passed to Subscribe. Trigger queries all Events
	// from the Event Store that have one of the configured Event names of the
	// Schedule. Trigger blocks until all Jobs have been applied or the Context
	// is canceled. When the Context is canceled, Trigger returns ctx.Err().
	//
	//	var bus event.Bus
	//	var store event.Store
	//	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"})
	//	errs, err := s.Subscribe(context.TODO(), func(j project.Job) error {
	//		log.Println("Triggered!")
	//		return nil
	//	})
	//	// handle err & errs
	//	_, err = s.Trigger(context.TODO())
	//	// handle err
	//	// Output: Triggered!
	Trigger(context.Context, ...TriggerOption) error
}

// SubscribeOption is a subscription option.
type SubscribeOption func(*subscribeConfig)

// ContinuousOption is an option for continuous projections.
type ContinuousOption func(*continously)

// PeriodicOption is an option for periodic projections.
type PeriodicOption func(*periodically)

// TriggerOption is an option for triggering a Schedule.
type TriggerOption func(*triggerConfig)

type subscribeConfig struct {
	filter event.Query
}

type triggerConfig struct {
	fully   bool
	queries []event.Query
}

type schedule struct {
	store      event.Store
	eventNames []string
}

type continously struct {
	schedule

	debounce time.Duration
	bus      event.Bus

	mux      sync.Mutex
	triggers []chan trigger
}

type periodically struct {
	schedule

	interval time.Duration

	mux      sync.Mutex
	triggers []chan trigger
}

type trigger struct {
	query event.Query
	cfg   triggerConfig
	wg    *sync.WaitGroup
}

// Filter returns an Option that adds a Query as a filter to a projection
// subscription. Only Events that would be included by the provided Query will
// be applied on a projection.
func Filter(filter event.Query) SubscribeOption {
	return func(c *subscribeConfig) {
		c.filter = filter
	}
}

// // FromBase returns an Option that ensures that a projection is built with every
// // Event from the past instead of just the new Events since the last time the
// // specific projection has been run.
// func FromBase() ApplyOption {
// 	return func(c *applyConfig) {
// 		c.fromBase = true
// 	}
// }

// Debounce returns a ContinuousOption that debounces received Events by the
// given Duration before a projection Job is created.
//
// Example:
//	var bus event.Bus
//	var store event.Store
//	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"}, project.Debounce(100*time.Millisecond))
//	errs, err := s.Subscribe(context.TODO())
//	// handle err & errs
func Debounce(d time.Duration) ContinuousOption {
	return func(c *continously) {
		c.debounce = d
	}
}

// TriggerFilter returns a TriggerOption that adds queries as filters to the
// event query for the events that are applied to a projection.
func TriggerFilter(queries ...event.Query) TriggerOption {
	return func(cfg *triggerConfig) {
		cfg.queries = append(cfg.queries, queries...)
	}
}

// Fully returns a TriggerOption that forces event queries for projections to
// ignore `ProjectionProgress` methods on projections and always query all
// events.
func Fully() TriggerOption {
	return func(cfg *triggerConfig) {
		cfg.fully = true
	}
}

// Continuously returns a Schedule that subscribes to the provided Event names
// and triggers a projection on every received Event.
//
// Example:
//	var bus event.Bus
//	var store event.Store
//
//	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"}, project.Debounce(100*time.Millisecond))
//	errs, err := s.Subscribe(context.TODO(), func(j project.Job) error {
//		foo := newFoo() // fetch or create the projection
//		if err := j.Apply(j.Context(), foo); err != nil {
//			return fmt.Errorf("apply Job: %w", err)
//		}
//		return nil
//	})
//	// handle err & errs
func Continuously(bus event.Bus, store event.Store, eventNames []string, opts ...ContinuousOption) Schedule {
	s := &continously{
		schedule: schedule{
			store:      store,
			eventNames: eventNames,
		},
		bus: bus,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Periodically returns a Schedule that periodically creates a projection Job
// every provided Duration interval. Only Events with one of the provided Event
// names are queried when applied to a projection.
//
// Example:
//	var store event.Store
//
//	s := project.Periodically(store, []string{"foo", "bar", "baz"})
//	errs, err := s.Subscribe(context.TODO(), func(j project.Job) error {
//		foo := newFoo() // fetch or create the projection
//		if err := j.Apply(j.Context(), foo); err != nil {
//			return fmt.Errorf("apply Job: %w", err)
//		}
//		return nil
//	})
//	// handle err & errs
func Periodically(store event.Store, interval time.Duration, eventNames []string, opts ...PeriodicOption) Schedule {
	s := &periodically{
		schedule: schedule{
			store:      store,
			eventNames: eventNames,
		},
		interval: interval,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *continously) Subscribe(ctx context.Context, applyFunc func(Job) error, opts ...SubscribeOption) (<-chan error, error) {
	cfg := configureSubscribe(opts...)
	events, errs, err := s.bus.Subscribe(ctx, s.eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v Events: %w", s.eventNames, err)
	}

	triggers := make(chan trigger)
	s.mux.Lock()
	s.triggers = append(s.triggers, triggers)
	s.mux.Unlock()

	go func() {
		<-ctx.Done()
		s.mux.Lock()
		defer s.mux.Unlock()
		for i, t := range s.triggers {
			if t == triggers {
				s.triggers = append(s.triggers[:i], s.triggers[i+1:]...)
				break
			}
		}
		close(triggers)
	}()

	out := make(chan error)

	var debounce *time.Timer

	var jobEventsMux sync.Mutex
	var jobEvents []event.Event

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() {
			if debounce != nil {
				debounce.Stop()
			}
		}()

		event.ForEvery(
			ctx,
			func(evt event.Event) {
				if !cfg.allowsEvent(evt) {
					return
				}

				jobEventsMux.Lock()
				jobEvents = append(jobEvents, evt)
				jobEventsMux.Unlock()

				apply := func() {
					jobEventsMux.Lock()
					evts := make([]event.Event, len(jobEvents))
					copy(evts, jobEvents)
					jobEvents = jobEvents[:0]
					jobEventsMux.Unlock()

					j := newContinuousJob(ctx, cfg, s.store, nil, evts, s.eventNames)
					if err := applyFunc(j); err != nil {
						select {
						case <-ctx.Done():
						case out <- fmt.Errorf("applyFunc: %w", err):
						}
					}
				}

				if s.debounce <= 0 {
					apply()
					return
				}

				if debounce != nil {
					debounce.Stop()
				}

				debounce = time.AfterFunc(s.debounce, apply)
			},
			func(err error) {
				select {
				case <-ctx.Done():
				case out <- fmt.Errorf("eventbus: %w", err):
				}
			},
			events, errs,
		)
	}()

	go func() {
		defer wg.Done()
		for t := range triggers {
			j := newContinuousJob(ctx, cfg, s.store, &t, nil, s.eventNames)
			if err := applyFunc(j); err != nil {
				select {
				case <-ctx.Done():
				case out <- fmt.Errorf("applyFunc: %w", err):
				}
			}
			t.wg.Done()
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

func (s *continously) Trigger(ctx context.Context, opts ...TriggerOption) error {
	cfg := newTriggerConfig(opts...)

	queries := append([]event.Query{
		query.New(
			query.Name(s.eventNames...),
			query.SortBy(event.SortTime, event.SortAsc),
		),
	}, cfg.queries...)

	q := query.Merge(queries...)

	s.mux.Lock()
	var wg sync.WaitGroup
	for _, triggers := range s.triggers {
		wg.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case triggers <- trigger{
			query: q,
			cfg:   cfg,
			wg:    &wg,
		}:
		}
	}
	s.mux.Unlock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *periodically) Subscribe(ctx context.Context, applyFunc func(Job) error, opts ...SubscribeOption) (<-chan error, error) {
	cfg := configureSubscribe(opts...)
	out := make(chan error)

	triggers := make(chan trigger)
	s.mux.Lock()
	s.triggers = append(s.triggers, triggers)
	s.mux.Unlock()

	go func() {
		<-ctx.Done()
		s.mux.Lock()
		defer s.mux.Unlock()
		for i, t := range s.triggers {
			if t == triggers {
				s.triggers = append(s.triggers[:i], s.triggers[i+1:]...)
				break
			}
		}
		close(triggers)
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				job := newPeriodicJob(ctx, cfg, s.store, s.eventNames, nil)
				if err := applyFunc(job); err != nil {
					select {
					case <-ctx.Done():
					case out <- fmt.Errorf("applyFunc: %w", err):
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for trigger := range triggers {
			j := newPeriodicJob(ctx, cfg, s.store, s.eventNames, &trigger)
			if err := applyFunc(j); err != nil {
				select {
				case <-ctx.Done():
				case out <- fmt.Errorf("applyFunc: %w", err):
				}
			}
			trigger.wg.Done()
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

func (s *periodically) Trigger(ctx context.Context, opts ...TriggerOption) error {
	cfg := newTriggerConfig(opts...)

	queries := append([]event.Query{
		query.New(
			query.Name(s.eventNames...),
			query.SortBy(event.SortTime, event.SortAsc),
		),
	}, cfg.queries...)

	q := query.Merge(queries...)

	s.mux.Lock()
	var wg sync.WaitGroup
	for _, triggers := range s.triggers {
		wg.Add(1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case triggers <- trigger{
			query: q,
			cfg:   cfg,
			wg:    &wg,
		}:
		}
	}
	s.mux.Unlock()

	wg.Wait()

	return nil
}

func configureSubscribe(opts ...SubscribeOption) subscribeConfig {
	var cfg subscribeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (cfg subscribeConfig) allowsEvent(evt event.Event) bool {
	return query.Test(cfg.filter, evt)
}

func newTriggerConfig(opts ...TriggerOption) triggerConfig {
	var cfg triggerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
