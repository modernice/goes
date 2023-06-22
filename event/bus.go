package event

import "context"

// #region bus
// Bus combines the capabilities of both a Publisher and a Subscriber, allowing
// it to send events to all subscribers and set up subscriptions for specific
// event names. It is an interface that embeds the Publisher and Subscriber
// interfaces.
type Bus interface {
	Publisher
	Subscriber
}

// Publisher is an interface that represents the ability to publish events to a
// message bus. It provides a single method, Publish, which takes a context and
// a variadic list of events and returns an error if the publishing process
// encounters any issues.
type Publisher interface {
	// Publish sends the given events to all subscribers of the event bus. It takes
	// a context and a variadic list of events as arguments and returns an error if
	// any occurred during the process.
	Publish(ctx context.Context, events ...Event) error
}

// Subscriber is an interface that provides a method for subscribing to specific
// events by their names. The Subscribe method returns a channel for receiving
// events, a channel for receiving errors, and an error if there are any issues
// during the subscription process.
type Subscriber interface {
	// Subscribe sets up a subscription for the specified event names, returning
	// channels for receiving events and errors, as well as an error if the
	// subscription fails. The context can be used to cancel the subscription.
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}

// #endregion bus

// Must wraps the given event and error channels, and panics if the provided
// error is not nil. It returns the same event and error channels if the error
// is nil. This function can be used to simplify error handling when setting up
// event subscriptions, ensuring that a valid subscription is established before
// proceeding.
//
// # Example
//
//	events := event.Must(bus.Subscribe(ctx, "event-name"))
func Must[D any](events <-chan Of[D], errs <-chan error, err error) (<-chan Of[D], <-chan error) {
	if err != nil {
		panic(err)
	}
	return events, errs
}
