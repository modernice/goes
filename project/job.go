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

// An EventApplier applies Events onto itself in order to build the state of a projection.
type EventApplier interface {
	ApplyEvent(event.Event)
}

// A Job is provided by a Projector when a projection should be run.
type Job interface {
	// Context returns the projection Context.
	Context() context.Context

	// Events returns the Events from the Job.
	Events(context.Context) ([]event.Event, error)

	// EventsFor returns the Events that would be applied to the given
	// projection. If the projection provides a `LatestEventTime` method, it is
	// used to only query Events that happened after that time.
	EventsFor(context.Context, EventApplier, ...ApplyOption) ([]event.Event, error)

	// Aggregates returns a map of Aggregate names to UUIDs, extracted from the
	// Events of the Job.
	Aggregates(context.Context) (map[string][]uuid.UUID, error)

	// AggregatesOf returns the UUIDs of Aggregates, extracted from the Events
	// of the Job.
	AggregatesOf(context.Context, string) ([]uuid.UUID, error)

	// Aggregate returns the first UUID of an Aggregate with the given name,
	// extracted from the Events of the Job.
	//
	// If no Event belongs to an Aggregate witht that name, uuid.Nil is returned.
	Aggregate(context.Context, string) (uuid.UUID, error)

	// Apply applies the projection on an EventApplier, which is usually a type
	// that embeds *Projection.
	Apply(context.Context, EventApplier, ...ApplyOption) error
}

// ApplyOption is an option for applying a projection Job.
type ApplyOption func(*applyConfig)

type latestEventTimeProvider interface {
	LatestEventTime() stdtime.Time
}

type postEventApplier interface {
	PostApplyEvent(event.Event)
}

type applyConfig struct {
	fromBase bool
}

// Apply projects the provided Events onto the given EventApplier, which in
// most cases is a type that embeds a *Projection.
func Apply(events []event.Event, proj EventApplier) error {
	postApplier, hasPostApply := proj.(postEventApplier)

	for _, evt := range events {
		proj.ApplyEvent(evt)
		if hasPostApply {
			postApplier.PostApplyEvent(evt)
		}
	}

	return nil
}

type continuousJob struct {
	ctx        context.Context
	cfg        subscribeConfig
	store      event.Store
	events     []event.Event
	eventNames []string
}

func newContinuousJob(
	ctx context.Context,
	cfg subscribeConfig,
	store event.Store,
	events []event.Event,
	eventNames []string,
) *continuousJob {
	return &continuousJob{
		ctx:        ctx,
		cfg:        cfg,
		store:      store,
		events:     events,
		eventNames: eventNames,
	}
}

func (j *continuousJob) Context() context.Context {
	return j.ctx
}

func (j *continuousJob) Events(ctx context.Context) ([]event.Event, error) {
	return j.EventsFor(ctx, nil)
}

func (j *continuousJob) EventsFor(ctx context.Context, p EventApplier, opts ...ApplyOption) ([]event.Event, error) {
	cfg := configureApply(opts...)

	if !cfg.fromBase {
		return j.events, nil
	}

	if ctx == nil {
		ctx = j.ctx
	}

	queryOpts := []query.Option{query.Name(j.eventNames...)}

	// If the projection provides a `LatestEventTime` method, use it to only
	// query Events that happened after that time.
	if p, ok := p.(latestEventTimeProvider); ok && !cfg.fromBase {
		if t := p.LatestEventTime(); !t.IsZero() {
			queryOpts = append(queryOpts, query.Time(time.After(t)))
		}
	}

	q := query.Merge(query.New(queryOpts...), j.cfg.filter)
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

func (j *continuousJob) Aggregates(ctx context.Context) (map[string][]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx)
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

func (j *continuousJob) AggregatesOf(ctx context.Context, name string) ([]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	aggregates, err := j.Aggregates(ctx)
	if err != nil {
		return nil, err
	}

	return aggregates[name], nil
}

func (j *continuousJob) Aggregate(ctx context.Context, name string) (uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	ids, err := j.AggregatesOf(ctx, name)
	if err != nil {
		return uuid.Nil, err
	}

	if len(ids) == 0 {
		return uuid.Nil, nil
	}

	return ids[0], nil
}

func (j *continuousJob) Apply(ctx context.Context, p EventApplier, opts ...ApplyOption) error {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.EventsFor(ctx, p, opts...)
	if err != nil {
		return err
	}

	return Apply(events, p)
}

type periodicJob struct {
	ctx        context.Context
	cfg        subscribeConfig
	store      event.Store
	eventNames []string
	cache      map[interface{}][]event.Event
}

func newPeriodicJob(ctx context.Context, cfg subscribeConfig, store event.Store, eventNames []string) *periodicJob {
	return &periodicJob{
		ctx:        ctx,
		cfg:        cfg,
		store:      store,
		eventNames: eventNames,
		cache:      make(map[interface{}][]event.Event),
	}
}

func (j *periodicJob) Context() context.Context {
	return j.ctx
}

func (j *periodicJob) Events(ctx context.Context) ([]event.Event, error) {
	return j.EventsFor(ctx, nil)
}

func (j *periodicJob) EventsFor(ctx context.Context, p EventApplier, opts ...ApplyOption) ([]event.Event, error) {
	cfg := configureApply(opts...)

	if ctx == nil {
		ctx = j.ctx
	}

	queryOpts := []query.Option{
		query.Name(j.eventNames...),
		query.SortByMulti(
			event.SortOptions{Sort: event.SortTime, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateName, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateID, Dir: event.SortAsc},
			event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
		),
	}

	if p, ok := p.(latestEventTimeProvider); ok && !cfg.fromBase {
		if t := p.LatestEventTime(); !t.IsZero() {
			queryOpts = append(queryOpts, query.Time(time.After(t)))
		}
	}

	q := query.New(queryOpts...)

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

	return events, nil
}

func (j *periodicJob) Aggregates(ctx context.Context) (map[string][]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.Events(ctx)
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

func (j *periodicJob) AggregatesOf(ctx context.Context, name string) ([]uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	aggregates, err := j.Aggregates(ctx)
	if err != nil {
		return nil, err
	}

	return aggregates[name], nil
}

func (j *periodicJob) Aggregate(ctx context.Context, name string) (uuid.UUID, error) {
	if ctx == nil {
		ctx = j.ctx
	}

	ids, err := j.AggregatesOf(ctx, name)
	if err != nil {
		return uuid.Nil, err
	}

	if len(ids) == 0 {
		return uuid.Nil, nil
	}

	return ids[0], nil
}

func (j *periodicJob) Apply(ctx context.Context, p EventApplier, opts ...ApplyOption) error {
	if ctx == nil {
		ctx = j.ctx
	}

	events, err := j.EventsFor(ctx, p, opts...)
	if err != nil {
		return err
	}

	return Apply(events, p)
}

func configureApply(opts ...ApplyOption) applyConfig {
	var cfg applyConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
