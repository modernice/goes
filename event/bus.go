package event

import "context"

// #region bus
// Bus is the pub-sub client for events.
type Bus interface {
	Publisher
	Subscriber
}

// A Publisher allows to publish events to subscribers of these events.
type Publisher interface {
	// Publish publishes events. Each event is sent to all subscribers of the event.
	Publish(ctx context.Context, events ...Event) error
}

// A Subscriber allows to subscribe to events.
type Subscriber interface {
	// Subscribe subscribes to events with the given names and returns two
	// channels – one for the received events and one for any asynchronous
	// errors that occur during the subscription. If Subscribe fails to
	// subscribe to all events, nil channels and an error are returned instead.
	//
	// When the provided context is canceled, the subscription is also canceled
	// and the returned channels are closed.
	Subscribe(ctx context.Context, names ...string) (<-chan Event, <-chan error, error)
}

// #endregion bus

// Must can be used to panic on failed event subscriptions:
//
//	var bus Bus
//	events, errs := Must(bus.Subscribe(context.TODO(), "foo", "bar", "baz"))
func Must[D any](events <-chan Of[D], errs <-chan error, err error) (<-chan Of[D], <-chan error) {
	if err != nil {
		panic(err)
	}
	return events, errs
}
