package projection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
)

const (
	// Triggered is the event name for triggering a Schedule.
	Triggered = "goes.projection.schedule.triggered"

	// TriggerAccepted is the event name for accepting a trigger.
	TriggerAccepted = "goes.projection.schedule.trigger_accepted"
)

var (
	// DefaultTriggerTimeout is the default timeout for triggering a Schedule.
	DefaultTriggerTimeout = 5 * time.Second

	// ErrUnhandledTrigger is returned when trying to trigger a Schedule that
	// isn't registered in a running Service.
	ErrUnhandledTrigger = errors.New("unhandled trigger")
)

// TriggeredData is the event data for triggering a Schedule.
type TriggeredData struct {
	TriggerID uuid.UUID
	Trigger   Trigger
	Schedule  string
}

// TriggerAcceptedData is the event data for accepting a trigger.
type TriggerAcceptedData struct {
	TriggerID uuid.UUID
}

// Service is an event-driven projection service. A Service allows to trigger
// Schedules that are registered in Services that communicate over the same
// event bus.
type Service struct {
	bus            event.Bus
	triggerTimeout time.Duration

	schedulesMux sync.RWMutex
	schedules    map[string]Schedule
}

// Schedule is a projection schedule.
type Schedule interface {
	// Subscribe subscribes the provided function to the Schedule and returns a
	// channel of asynchronous projection errors. When the Schedule is
	// triggered, a Job is created and passed to subscribers of the Schedule.
	// Errors returned from subscribers are pushed into the returned error
	// channel.
	//
	//	var proj projection.Projection // created by yourself
	//	s := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	//	errs, err := s.Subscribe(context.TODO(), func(job projection.Job) error {
	//		return job.Apply(job, proj) // job.Apply applies the appropriate events onto the projection
	//	})
	//	// handle err
	//	for err := range errs {
	//		log.Printf("projection failed: %v\n", err)
	//	}
	Subscribe(context.Context, func(Job) error) (<-chan error, error)

	// Trigger manually triggers the Schedule immediately. A Job is created and
	// passed to every subscriber of the Schedule. Trigger does not wait for the
	// Job to be handled by the subscribers.
	//
	// Reset projections
	//
	// The created Job can be configured to reset Projections before applying
	// events onto them, effectively rebuilding the entire projection from the
	// beginning (first event):
	//
	//	var s projection.Schedule
	//	err := s.Trigger(context.TODO(), projection.Reset())
	//
	// When a Projection implements progressor (or embeds *Progressor), the
	// progress time of the Projection is set to 0.
	//
	// When a Projection has a `Reset()` method, that method is called to allow
	// for custom reset logic. Implementers of Projection should appropriately
	// reset the state of the Projection.
	//
	// Custom event query
	//
	// When a Job is created, it is passed an event query to fetch the events
	// for the Projections. By default, this query fetches the events configured
	// in the Schedule sorted by time. A custom query may be provided using the
	// Query option. Don't forget to configure correct sorting when providing a
	// custom query:
	//
	//	var s projection.Schedule
	//	err := s.Trigger(context.TODO(), projection.Query(query.New(
	//		query.AggregateName("foo", "bar"),
	//		query.SortBy(event.SortTime, event.SortAsc),
	//	)))
	//
	// Event filters
	//
	// Queried events can be further filtered using the Filter option. Filters
	// are applied in-memory, after the events have already been fetched from
	// the event store. When multiple filters are passed, events must match
	// against every filter to be applied to the Projections. Sorting options of
	// filters are ignored.
	//
	//	var s projection.Schedule
	//	err := s.Trigger(context.TODO(), projection.Filter(query.New(...), query.New(...)))
	Trigger(context.Context, ...TriggerOption) error
}

// RegisterService register the projection service events into an event
// registry.
func RegisterService(r *codec.Registry) {
	gob := codec.Gob(r)
	gob.GobRegister(Triggered, func() any { return TriggeredData{} })
	gob.GobRegister(TriggerAccepted, func() any { return TriggerAcceptedData{} })
}

// ServiceOption is an option for creating a Service.
type ServiceOption func(*Service)

// RegisterSchedule returns a ServiceOption that registers the Schedule s with
// the given name into a Service
func RegisterSchedule(name string, s Schedule) ServiceOption {
	return func(svc *Service) {
		svc.schedules[name] = s
	}
}

// TriggerTimeout returns a ServiceOption that overrides the default timeout for
// triggering a Schedule. Default is 5s. Zero Duration means no timeout.
func TriggerTimeout(d time.Duration) ServiceOption {
	return func(svc *Service) {
		svc.triggerTimeout = d
	}
}

// NewService returns a new Service.
//
//	var bus event.Bus
//	var fooSchedule projection.Schedule
//	var barSchedule projection.Schedule
//	svc := NewService(
//		bus,
//		projection.RegisterSchedule("foo", fooSchedule),
//		projection.RegisterSchedule("bar", barSchedule),
//	)
//
//	errs, err := svc.Run(context.TODO())
func NewService(bus event.Bus, opts ...ServiceOption) *Service {
	svc := Service{
		bus:            bus,
		triggerTimeout: DefaultTriggerTimeout,
		schedules:      make(map[string]Schedule),
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return &svc
}

// Register registers a Schedule with the given name into the Service.
func (svc *Service) Register(name string, s Schedule) {
	svc.schedulesMux.Lock()
	svc.schedules[name] = s
	svc.schedulesMux.Unlock()
}

// Trigger triggers the Schedule with the given name.
//
// Trigger publishes a Triggered event over the event bus and waits for a
// TriggerAccepted event to be published by another Service. Should the
// TriggerAccepted event not be published within the trigger timeout,
// ErrUnhandledTrigger is returned. When ctx is canceled, ctx.Err() is returned.
func (svc *Service) Trigger(ctx context.Context, name string, opts ...TriggerOption) error {
	events, errs, err := svc.bus.Subscribe(ctx, TriggerAccepted)
	if err != nil {
		return fmt.Errorf("subscribe to %q Event: %w", TriggerAccepted, err)
	}

	id := uuid.New()
	evt := event.New[any](Triggered, TriggeredData{
		TriggerID: id,
		Trigger:   NewTrigger(opts...),
		Schedule:  name,
	})
	if err := svc.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q Event: %w", evt.Name(), err)
	}

	if svc.triggerTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, svc.triggerTimeout)
		defer cancel()
	}

	done := errors.New("done")
	if err := streams.Walk(ctx, func(evt event.Event) error {
		data := evt.Data().(TriggerAcceptedData)
		if data.TriggerID != id {
			return nil
		}
		return done
	}, events, errs); !errors.Is(err, done) {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrUnhandledTrigger
		}
		return fmt.Errorf("eventbus: %w", err)
	}

	return nil
}

// Run starts the Service is a new goroutine and returns a channel of
// asynchronous errors, or a single error if the event bus fails to subscribe.
// When another Service triggers a Schedule with a name that is registered in
// svc, svc accepts that trigger by publishing a TriggerAccepted event and then
// actually triggers the Schedule.
func (svc *Service) Run(ctx context.Context) (<-chan error, error) {
	events, errs, err := svc.bus.Subscribe(ctx, Triggered)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %q Event: %w", Triggered, err)
	}

	out := make(chan error)
	go svc.handleEvents(ctx, events, errs, out)

	return out, nil
}

func (svc *Service) handleEvents(ctx context.Context, events <-chan event.Event, errs <-chan error, out chan<- error) {
	defer close(out)

	fail := func(err error) {
		select {
		case <-ctx.Done():
		case out <- err:
		}
	}

	streams.ForEach(ctx, func(evt event.Event) {
		data := evt.Data().(TriggeredData)

		s, ok := svc.schedule(data.Schedule)
		if !ok {
			return
		}

		evt = event.New[any](TriggerAccepted, TriggerAcceptedData{TriggerID: data.TriggerID})
		if err := svc.bus.Publish(ctx, evt); err != nil {
			fail(fmt.Errorf("publish %q Event: %w", evt.Name(), err))
			return
		}

		if err := s.Trigger(ctx, data.Trigger.Options()...); err != nil {
			fail(fmt.Errorf("trigger %q schedule: %w", data.Schedule, err))
		}
	}, fail, events, errs)
}

func (svc *Service) schedule(name string) (Schedule, bool) {
	svc.schedulesMux.RLock()
	s, ok := svc.schedules[name]
	svc.schedulesMux.RUnlock()
	return s, ok
}
