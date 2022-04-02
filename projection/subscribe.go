package projection

import (
	"context"

	"github.com/modernice/goes/event"
)

// SubscribeOption is an option for Schedule.Subscribe.
type SubscribeOption func(*Subscription)

// Subscripion is the configuration for a subscription to a projection schedule.
type Subscription struct {
	// If provided, the projection schedule triggers a projection job on startup.
	// The projection job's `Aggregates()` and `Aggregate()` helpers will use
	// the query of this trigger to extract the aggregates from the event store.
	// This allows to optimize the query performance of initial projection runs,
	// which often times need to fetch all ids of specific aggregates from the
	// event store in order to get all projections up-to-date.
	Startup *Trigger

	// BeforeEvent are the "before"-interceptors for the event streams created
	// by a job's `EventsFor()` and `Apply()` methods.
	BeforeEvent []func(context.Context, event.Event) ([]event.Event, error)
}

// Startup returns a SubscribeOption that triggers an initial projection run
// when subscribing to a projection schedule.
func Startup(opts ...TriggerOption) SubscribeOption {
	return func(s *Subscription) {
		t := NewTrigger(opts...)
		s.Startup = &t
	}
}

// BeforeEvent returns a SubscribeOption that registers the given function as a
// "before"-interceptor for the event streams created by a job's `EventsFor()`
// and `Apply()` methods. For each received event of a stream that has one of
// the provided event names, the provided function is called, and the returned
// events of that function are inserted into the stream before the intercepted
// event. If no event names are provided, the interceptor is applied to all events.
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

// NewSubscription creates a Subscription using the provided options.
func NewSubscription(opts ...SubscribeOption) Subscription {
	var sub Subscription
	for _, opt := range opts {
		opt(&sub)
	}
	return sub
}
