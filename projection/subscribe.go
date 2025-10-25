package projection

import (
	"context"

	"github.com/modernice/goes/event"
)

// SubscribeOption configures Schedule.Subscribe.
type SubscribeOption func(*Subscription)

// Subscription holds subscription configuration.
type Subscription struct {
	// Startup triggers an initial job when subscribing.
	Startup *Trigger

	// BeforeEvent interceptors can inject events before processed ones.
	BeforeEvent []func(context.Context, event.Event) ([]event.Event, error)
}

// Startup triggers a job as soon as the subscription is established.
func Startup(opts ...TriggerOption) SubscribeOption {
	return func(s *Subscription) {
		t := NewTrigger(opts...)
		s.Startup = &t
	}
}

// BeforeEvent registers fn to insert events before matched ones. If events is
// empty it applies to all events.
func BeforeEvent[Data any](fn func(context.Context, event.Of[Data]) ([]event.Event, error), events ...string) SubscribeOption {
	return func(s *Subscription) {
		s.BeforeEvent = append(s.BeforeEvent, func(ctx context.Context, e event.Event) ([]event.Event, error) {
			if len(events) == 0 {
				return fn(ctx, event.Cast[Data](e))
			}

			for _, name := range events {
				if name == e.Name() {
					return fn(ctx, event.Cast[Data](e))
				}
			}

			return nil, nil
		})
	}
}

// NewSubscription builds a Subscription from opts.
func NewSubscription(opts ...SubscribeOption) Subscription {
	var sub Subscription
	for _, opt := range opts {
		opt(&sub)
	}
	return sub
}
