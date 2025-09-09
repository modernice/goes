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
	// Triggered signals a request to trigger a schedule.
	Triggered = "goes.projection.schedule.triggered"

	// TriggerAccepted acknowledges a trigger request.
	TriggerAccepted = "goes.projection.schedule.trigger_accepted"
)

var (
	// DefaultTriggerTimeout is the default wait for a trigger acknowledgement.
	DefaultTriggerTimeout = 5 * time.Second

	// ErrUnhandledTrigger reports that no service accepted the trigger.
	ErrUnhandledTrigger = errors.New("unhandled trigger")
)

// TriggeredData describes a schedule trigger request.
type TriggeredData struct {
	TriggerID uuid.UUID
	Trigger   Trigger
	Schedule  string
}

// TriggerAcceptedData is emitted when a trigger is accepted.
type TriggerAcceptedData struct {
	TriggerID uuid.UUID
}

// Service coordinates schedule triggers over an event bus.
type Service struct {
	bus            event.Bus
	triggerTimeout time.Duration

	schedulesMux sync.RWMutex
	schedules    map[string]Schedule
}

// Schedule defines a source of projection jobs.
type Schedule interface {
	// Subscribe invokes fn for every triggered job.
	Subscribe(context.Context, func(Job) error, ...SubscribeOption) (<-chan error, error)

	// Trigger creates a job immediately and delivers it to subscribers.
	Trigger(context.Context, ...TriggerOption) error
}

// RegisterService register the projection service events into an event registry.
func RegisterService(r codec.Registerer) {
	codec.Register[TriggeredData](r, Triggered)
	codec.Register[TriggerAcceptedData](r, TriggerAccepted)
}

// ServiceOption is an option for creating a Service.
type ServiceOption func(*Service)

// RegisterSchedule registers s under name when creating a Service.
func RegisterSchedule(name string, s Schedule) ServiceOption {
	return func(svc *Service) {
		svc.schedules[name] = s
	}
}

// TriggerTimeout sets the trigger acknowledgement timeout. Zero disables it.
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

// Register adds s under name.
func (svc *Service) Register(name string, s Schedule) {
	svc.schedulesMux.Lock()
	svc.schedules[name] = s
	svc.schedulesMux.Unlock()
}

// Trigger emits a trigger request for the named schedule and waits for an
// acknowledgement. ErrUnhandledTrigger is returned if none arrives before the
// timeout.
func (svc *Service) Trigger(ctx context.Context, name string, opts ...TriggerOption) error {
	events, errs, err := svc.bus.Subscribe(ctx, TriggerAccepted)
	if err != nil {
		return fmt.Errorf("subscribe to %q event: %w", TriggerAccepted, err)
	}

	id := uuid.New()
	evt := event.New[any](Triggered, TriggeredData{
		TriggerID: id,
		Trigger:   NewTrigger(opts...),
		Schedule:  name,
	})
	if err := svc.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
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

// Run listens for trigger events and dispatches them to registered schedules.
func (svc *Service) Run(ctx context.Context) (<-chan error, error) {
	events, errs, err := svc.bus.Subscribe(ctx, Triggered)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %q event: %w", Triggered, err)
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
			fail(fmt.Errorf("publish %q event: %w", evt.Name(), err))
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
