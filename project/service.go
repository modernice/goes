package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/concurrent"
)

const (
	// ScheduleTriggerRequested is the event that is published when triggering a Schedule
	// through a Service.
	ScheduleTriggerRequested = "goes.project.service.schedule_trigger_requested"

	// ScheduleTriggered is the event that is published when a Service received
	// a ScheduleTriggerRequested event and actually triggered the Schedule.
	ScheduleTriggered = "goes.project.service.schedule_triggered"
)

var (
	// DefaultTriggerTimeout is the default timeout for triggering a Schedule
	// through a Service.
	DefaultTriggerTimeout = 3 * time.Second
)

var (
	// ErrUnhandledTrigger is returned when triggering a Schedule through a Service
	// and no (other) Service has a Schedule with that name registered.
	ErrUnhandledTrigger = errors.New("unhandled trigger")
)

// ScheduleTrigggeredData is the event data for the ScheduleTriggeredRequested event.
type ScheduleTriggeredRequestedData struct {
	// TriggerID is a randomly generated UUID that is used to identify the
	// trigger across Services.
	TriggerID uuid.UUID

	// Name is the name of the Schedule.
	Name string

	// Config is the trigger configuration.
	Config TriggerConfig
}

// ScheduleTriggeredData is the event data for the ScheduleTriggered event.
type ScheduleTriggeredData struct {
	// TriggerID is the UUID of the trigger.
	TriggerID uuid.UUID

	// Name is the name of the triggered Schedule.
	Name string

	// Config is the used trigger configuration.
	Config TriggerConfig

	// TriggerError is the error returned from triggering the Schedule, or an
	// empty string if it returned no error.
	TriggerError string
}

// Service is an event-driven projection service that allows to trigger
// Schedules in services that communicate over the same event bus.
type Service struct {
	bus       event.Bus
	schedules map[string]Schedule
}

// ServiceOption is an option for a Service.
type ServiceOption func(*Service)

// TriggerOption is an option for triggering a Schedule.
type TriggerOption func(*TriggerConfig)

// TriggerConfig is the configuration for triggering a Schedule.
type TriggerConfig struct {
	Fully   bool
	Timeout time.Duration
}

// RegisterSchedule returns a ServiceOption that registers the Schedule s in the
// Service with the provided name.
//
//	var cbus command.Bus
//	var schedule project.Schedule
//	svc := project.NewService(cbus, project.RegisterSchedule("example", schedule))
func RegisterSchedule(name string, s Schedule) ServiceOption {
	return func(svc *Service) {
		svc.schedules[name] = s
	}
}

// Fully returns a TriggerOption that forces event queries for projections to
// ignore `ProjectionProgress` methods on projections and always query all
// events.
func Fully() TriggerOption {
	return func(cfg *TriggerConfig) {
		cfg.Fully = true
	}
}

// TriggerTimeout returns a TriggerOption that overrides the default timeout for
// triggering a Schedule.
func TriggerTimeout(d time.Duration) TriggerOption {
	return func(cfg *TriggerConfig) {
		cfg.Timeout = d
	}
}

// NewService returns an event-driven projection service. It registers the
// ScheduleTriggered event into reg. Use the RegisterSchedule ServiceOption to
// register Schedules with a name into the Service.
//
//	var reg event.Registry
//	var bus event.Bus
//	var store event.Store
//	schedule := project.Continuously(bus, store, []string{"foo", "bar", "baz"})
//	svc := project.NewService(reg, bus, project.RegisterSchedule("example", schedule))
func NewService(reg event.Registry, bus event.Bus, opts ...ServiceOption) *Service {
	registerEvents(reg)
	svc := Service{
		bus:       bus,
		schedules: make(map[string]Schedule),
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return &svc
}

// Run starts the service in a new goroutine and returns a channel of
// asynchronous errors. The channel is closed when ctx is canceled.
//
// Run listens for triggers of Schedules that are registered in the Service and
// actually triggers them.
func (svc *Service) Run(ctx context.Context) (<-chan error, error) {
	events, errs, err := svc.bus.Subscribe(ctx, ScheduleTriggerRequested)
	if err != nil {
		return nil, fmt.Errorf("subscribe to %q event: %w", ScheduleTriggerRequested, err)
	}

	out, fail := concurrent.Errors(ctx)

	go func() {
		defer close(out)
		event.ForEvery(ctx, svc.eventHandler(ctx, fail), fail, events, errs)
	}()

	return out, nil
}

func (svc *Service) eventHandler(ctx context.Context, fail func(err error)) func(event.Event) {
	return func(evt event.Event) {
		data := evt.Data().(ScheduleTriggeredRequestedData)

		schedule, ok := svc.schedules[data.Name]
		if !ok {
			return
		}

		err := schedule.trigger(ctx, data.Config)
		if err != nil {
			fail(fmt.Errorf("trigger %q Schedule: %w", data.Name, err))
			return
		}

		var errmsg string
		if err != nil {
			errmsg = err.Error()
		}

		evt = event.New(ScheduleTriggered, ScheduleTriggeredData{
			TriggerID:    data.TriggerID,
			Name:         data.Name,
			Config:       data.Config,
			TriggerError: errmsg,
		})

		if err := svc.bus.Publish(ctx, evt); err != nil {
			fail(fmt.Errorf("publish %q event: %w", evt.Name(), err))
		}
	}
}

// Trigger triggers the Schedule with the given name.
func (svc *Service) Trigger(ctx context.Context, name string, opts ...TriggerOption) error {
	cfg := newTriggerConfig(opts...)

	events, errs, err := svc.bus.Subscribe(ctx, ScheduleTriggered)
	if err != nil {
		return fmt.Errorf("subscribe to %q event: %w", ScheduleTriggered, err)
	}

	triggerID := uuid.New()

	evt := event.New(ScheduleTriggerRequested, ScheduleTriggeredRequestedData{
		TriggerID: triggerID,
		Name:      name,
		Config:    cfg,
	})

	if err := svc.bus.Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish %q event: %w", evt.Name(), err)
	}

	walkCtx, cancelWalk := context.WithTimeout(ctx, cfg.Timeout)
	defer cancelWalk()

	walkError := make(chan error)
	triggerError := make(chan error)

	go func() {
		if err := event.Walk(walkCtx, func(evt event.Event) error {
			data := evt.Data().(ScheduleTriggeredData)
			if data.TriggerID != triggerID {
				return nil
			}

			defer cancelWalk()

			var err error
			if data.TriggerError != "" {
				err = errors.New(data.TriggerError)
			}

			select {
			case <-walkCtx.Done():
				return walkCtx.Err()
			case triggerError <- err:
			}

			return nil
		}, events, errs); err != nil {
			select {
			case <-ctx.Done():
			case walkError <- err:
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-walkError:
			if errors.Is(err, context.DeadlineExceeded) {
				return ErrUnhandledTrigger
			}
		case err := <-triggerError:
			return err
		}
	}
}

func registerEvents(reg event.Registry) {
	reg.Register(ScheduleTriggerRequested, func() event.Data { return ScheduleTriggeredRequestedData{} })
}

func newTriggerConfig(opts ...TriggerOption) TriggerConfig {
	var cfg TriggerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTriggerTimeout
	}
	return cfg
}
