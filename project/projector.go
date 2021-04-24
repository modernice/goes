package project

import (
	"context"
	"fmt"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/query/time"
)

// A Projector projects Events on user-defined projections.
type Projector struct {
	bus   event.Bus
	store event.Store
}

// An EventApplier applies Events onto itself in order to build the state of a projection.
type EventApplier interface {
	ApplyEvent(event.Event)
}

// A Job is provided by a Projector when a projection should be run.
type Job interface {
	// Context returns the projection Context.
	Context() context.Context

	// Events returns the Events that would be applied onto the given projection.
	Events(context.Context, interface{}) ([]event.Event, error)

	// Aggregates returns a map of Aggregate names to UUIDs, extracted from the
	// Events that would be applied to the given projection.
	Aggregates(context.Context, interface{}) (map[string][]uuid.UUID, error)

	// AggregatesOf returns the UUIDs of Aggregates, extracted from the Events
	// that would be applied to the given projection.
	AggregatesOf(context.Context, string, interface{}) ([]uuid.UUID, error)

	// Aggregate returns the first UUID of an Aggregate with the given name,
	// extracted from the Events that would be applied to the given projection.
	// If no Event belongs to an Aggregate witht that name, uuid.Nil is returned.
	Aggregate(context.Context, string, interface{}) (uuid.UUID, error)

	// Apply applies the projection on an EventApplier, which is usually a type
	// that embeds *Projection.
	Apply(context.Context, EventApplier) error
}

// Option is a subscription option.
type Option func(*config)

type config struct {
	filter   event.Query
	fromBase bool
}

type latestEventTimeProvider interface {
	LatestEventTime() stdtime.Time
}

type postEventApplier interface {
	PostApplyEvent(event.Event)
}

// Filter returns an Option that adds a Query as a filter to a projection
// subscription. Only Events that would be included by the provided Query will
// be applied on a projection.
func Filter(filter event.Query) Option {
	return func(c *config) {
		c.filter = filter
	}
}

// FromBase returns an Option that ensures that a projection is built with every
// Event from the past instead of just the new Events since the last time a
// specific projection has been run.
func FromBase() Option {
	return func(c *config) {
		c.fromBase = true
	}
}

// NewProjector returns a Projector. A Projector subscribes to Events and
// applies them on projections in order to build them.
func NewProjector(bus event.Bus, store event.Store) *Projector {
	return &Projector{
		bus:   bus,
		store: store,
	}
}

// Project projects the provided Events onto the given EventApplier, which in
// most cases is a type that embeds a *Projection.
func (p *Projector) Project(ctx context.Context, events []event.Event, proj EventApplier) error {
	postApplier, hasPostApply := proj.(postEventApplier)

	for _, evt := range events {
		proj.ApplyEvent(evt)
		if hasPostApply {
			postApplier.PostApplyEvent(evt)
		}
	}

	return nil
}

// Continously subscribes to the provided Event names and calls applyFunc with
// a projection Job everytime one of the Events is published. When the
// subscription to the underlying Event Bus receives an error, that error is
// passed to the returned error channel. Similarly, when applyFunc returns a
// non-nil error, that error is also passed to the returned error channel.
// The caller must ensure to read from the returned error channel to prevent
// goroutines from blocking.
//
// Use the provided Job to apply new Events onto a projection:
//
//	var proj project.Projector
//	errs, err := proj.Continuously(context.TODO(), "foo", "bar", "baz", func(job project.job) error {
//		var foo exampleProjection // instantiate your concrete type
//		if err := job.Apply(foo); err != nil {
//			return fmt.Errorf("apply projection: %w", err)
//		}
//		return nil
//	})
// 	// handle err & errs
func (p *Projector) Continuously(
	ctx context.Context,
	eventNames []string,
	applyFunc func(Job) error,
	opts ...Option,
) (<-chan error, error) {
	events, errs, err := p.bus.Subscribe(ctx, eventNames...)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %v Events: %w", eventNames, err)
	}

	out := make(chan error)
	go p.continuously(ctx, configure(opts...), eventNames, applyFunc, events, errs, out)

	return out, nil
}

func (p *Projector) continuously(
	ctx context.Context,
	cfg config,
	eventNames []string,
	applyFunc func(Job) error,
	events <-chan event.Event,
	errs <-chan error,
	out chan<- error,
) {
	defer close(out)
	event.ForEvery(
		func(evt event.Event) {
			if cfg.allowsEvent(evt) {
				job := p.newContinuousJob(ctx, cfg, p.store, evt, eventNames)
				if err := applyFunc(job); err != nil {
					select {
					case <-ctx.Done():
					case out <- fmt.Errorf("apply Projection: %w", err):
					}
				}
			}
		},
		func(err error) {
			select {
			case <-ctx.Done():
			case out <- fmt.Errorf("eventbus: %w", err):
			}
		},
		events, errs,
	)
}

// Periodically queries the Event Store every interval for the given Event names
// and calls applyFunc with a projection Job. Errors from the underlying Event
// Store and errors returned by applyFunc are passed to the returned error
// channel. The caller must ensure to read from the returned error channel to
// prevent goroutines from blocking.
func (p *Projector) Periodically(
	ctx context.Context,
	interval stdtime.Duration,
	eventNames []string,
	applyFunc func(Job) error,
	opts ...Option,
) (<-chan error, error) {
	out := make(chan error)
	go p.periodically(ctx, configure(opts...), interval, eventNames, applyFunc, out)
	return out, nil
}

func (p *Projector) periodically(
	ctx context.Context,
	cfg config,
	interval stdtime.Duration,
	eventNames []string,
	applyFunc func(Job) error,
	out chan<- error,
) {
	defer close(out)

	ticker := stdtime.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job := p.newPeriodicJob(ctx, cfg, p.store, eventNames)
			if err := applyFunc(job); err != nil {
				select {
				case <-ctx.Done():
				case out <- fmt.Errorf("apply Projection: %w", err):
				}
			}
		}
	}
}

type continuousJob struct {
	ctx        context.Context
	projector  *Projector
	cfg        config
	store      event.Store
	evt        event.Event
	eventNames []string
	cache      []event.Event
}

func (p *Projector) newContinuousJob(
	ctx context.Context,
	cfg config,
	store event.Store,
	evt event.Event,
	eventNames []string,
) *continuousJob {
	return &continuousJob{
		ctx:        ctx,
		projector:  p,
		cfg:        cfg,
		store:      store,
		evt:        evt,
		eventNames: eventNames,
	}
}

func (j *continuousJob) Context() context.Context {
	return j.ctx
}

func (j *continuousJob) Events(ctx context.Context, p interface{}) ([]event.Event, error) {
	if j.cache != nil {
		return j.cache, nil
	}

	if ctx == nil {
		ctx = j.ctx
	}

	// If the projection shouldn't be run with past Events, return the single
	// received Event.
	if !j.cfg.fromBase {
		if !j.cfg.allowsEvent(j.evt) {
			return []event.Event{}, nil
		}
		return []event.Event{j.evt}, nil
	}

	opts := []query.Option{query.Name(j.eventNames...)}

	// If the projection provides a `LatestEventTime` method, use it to only
	// query Events that happened after that time.
	if p, ok := p.(latestEventTimeProvider); ok && !j.cfg.fromBase {
		if t := p.LatestEventTime(); !t.IsZero() {
			opts = append(opts, query.Time(time.After(t)))
		}
	}

	q := query.Merge(query.New(opts...), j.cfg.filter)
	str, errs, err := j.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query Events: %w", err)
	}

	events, err := event.Drain(ctx, str, errs)
	if err != nil {
		return nil, fmt.Errorf("drain Events: %w", err)
	}

	return events, nil
}

func (j *continuousJob) Aggregates(ctx context.Context, p interface{}) (map[string][]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx, p)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]uuid.UUID, len(events))
	for _, evt := range events {
		if evt.AggregateName() != "" {
			out[evt.AggregateName()] = append(out[evt.AggregateName()], evt.AggregateID())
		}
	}

	return out, nil
}

func (j *continuousJob) AggregatesOf(ctx context.Context, name string, p interface{}) ([]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	aggregates, err := j.Aggregates(ctx, p)
	if err != nil {
		return nil, err
	}

	return aggregates[name], nil
}

func (j *continuousJob) Aggregate(ctx context.Context, name string, p interface{}) (uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	ids, err := j.AggregatesOf(ctx, name, p)
	if err != nil {
		return uuid.Nil, err
	}

	if len(ids) == 0 {
		return uuid.Nil, nil
	}

	return ids[0], nil
}

func (j *continuousJob) Apply(ctx context.Context, p EventApplier) error {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx, p)
	if err != nil {
		return err
	}

	return j.projector.Project(ctx, events, p)
}

type periodicJob struct {
	ctx        context.Context
	cfg        config
	store      event.Store
	projector  *Projector
	eventNames []string
	cache      map[interface{}][]event.Event
}

func (p *Projector) newPeriodicJob(ctx context.Context, cfg config, store event.Store, eventNames []string) *periodicJob {
	return &periodicJob{
		ctx:        ctx,
		cfg:        cfg,
		store:      store,
		projector:  p,
		eventNames: eventNames,
		cache:      make(map[interface{}][]event.Event),
	}
}

func (j *periodicJob) Context() context.Context {
	return j.ctx
}

func (j *periodicJob) Events(ctx context.Context, p interface{}) ([]event.Event, error) {
	if events, ok := j.cache[p]; ok {
		return events, nil
	}

	if ctx == nil {
		ctx = j.ctx
	}

	opts := []query.Option{
		query.Name(j.eventNames...),
		query.SortByMulti(
			event.SortOptions{Sort: event.SortTime, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	}

	// If the projection provides a `LatestEventTime` method, use it to only
	// query Events that happened after that time.
	if p, ok := p.(latestEventTimeProvider); ok && !j.cfg.fromBase {
		if t := p.LatestEventTime(); !t.IsZero() {
			opts = append(opts, query.Time(time.After(t)))
		}
	}

	q := query.New(opts...)

	if j.cfg.filter != nil {
		q = query.Merge(q, j.cfg.filter)
	}

	str, errs, err := j.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query Events: %w", err)
	}

	events, err := event.Drain(ctx, str, errs)
	if err != nil {
		return nil, fmt.Errorf("drain Events: %w", err)
	}

	j.cache[p] = events

	return events, nil
}

func (j *periodicJob) Aggregates(ctx context.Context, p interface{}) (map[string][]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx, p)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]uuid.UUID, len(events))
	for _, evt := range events {
		if evt.AggregateName() != "" {
			out[evt.AggregateName()] = append(out[evt.AggregateName()], evt.AggregateID())
		}
	}

	return out, nil
}

func (j *periodicJob) AggregatesOf(ctx context.Context, name string, p interface{}) ([]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	aggregates, err := j.Aggregates(ctx, p)
	if err != nil {
		return nil, err
	}

	return aggregates[name], nil
}

func (j *periodicJob) Aggregate(ctx context.Context, name string, p interface{}) (uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	ids, err := j.AggregatesOf(ctx, name, p)
	if err != nil {
		return uuid.Nil, err
	}

	if len(ids) == 0 {
		return uuid.Nil, nil
	}

	return ids[0], nil
}

func (j *periodicJob) Apply(ctx context.Context, p EventApplier) error {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx, p)
	if err != nil {
		return err
	}

	return j.projector.Project(j.ctx, events, p)
}

func configure(opts ...Option) config {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (cfg config) allowsEvent(evt event.Event) bool {
	return query.Test(cfg.filter, evt)
}
