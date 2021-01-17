package event

import "context"

// Bus is the pub-sub client for Events.
type Bus interface {
	// Publish should send each Event evt in events to subscribers who
	// subscribed to Events with a name of evt.Name().
	Publish(ctx context.Context, events ...Event) error

	// Subscribe returns a channel of Events. For every published Event evt
	// where evt.Name() is one of names, that Event should be received from the
	// returned Event channel. When ctx is canceled, events channel should be
	// closed by the implementing Bus.
	Subscribe(ctx context.Context, names ...string) (<-chan Event, error)
}
