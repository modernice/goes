package event

//go:generate mockgen -source=bus.go -destination=./mocks/bus.go

import "context"

// Bus is the pub-sub client for events.
type Bus[D any] interface {
	// Publish publishes the given events to subscribers of those events.
	Publish(ctx context.Context, events ...EventOf[D]) error

	// Subscribe returns a channel of Events and a channel of asynchronous errors.
	// Only Events whose name is one of the provided names will be received from the
	// returned Event channel.
	//
	// When Subscribe fails to create the subscription, the returned channels
	// are nil and an error is returned.
	//
	// When ctx is canceled, both the Event and error channel are closed.
	//
	// Errors
	//
	// Callers of Subscribe must ensure that errors are received from the
	// returned error channel; otherwise the Bus may be blocked by the error
	// channel.
	Subscribe(ctx context.Context, names ...string) (<-chan EventOf[D], <-chan error, error)
}

// Must can be used to panic on failed event subscriptions:
//
//	var bus Bus
//	events, errs := Must(bus.Subscribe(context.TODO(), "foo", "bar", "baz"))
func Must[D any](events <-chan EventOf[D], errs <-chan error, err error) (<-chan EventOf[D], <-chan error) {
	if err != nil {
		panic(err)
	}
	return events, errs
}
