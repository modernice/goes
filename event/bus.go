package event

import "context"

// Bus combines Publisher and Subscriber. Implementations publish events and
// allow consumers to subscribe to specific event names.
type Bus interface {
	Publisher
	Subscriber
}

// Publisher publishes events to a bus.
type Publisher interface {
	// Publish sends events to all subscribers.
	Publish(ctx context.Context, events ...Event) error
}

// Subscriber subscribes to named events.
type Subscriber interface {
	// Subscribe registers interest in names and returns channels for events
	// and errors. The context is used to cancel the subscription.
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}

// Must returns events and errs or panics if err is non-nil. It is a convenience
// helper for subscriptions.
func Must[D any](events <-chan Of[D], errs <-chan error, err error) (<-chan Of[D], <-chan error) {
	if err != nil {
		panic(err)
	}
	return events, errs
}
