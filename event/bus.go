package event

//go:generate mockgen -source=bus.go -destination=./mocks/bus.go

import "context"

// Bus is the pub-sub client for Events.
type Bus interface {
	// Publish should send the Event to subscribers who subscribed to Events
	// whose name is evt.Name().
	Publish(ctx context.Context, evt Event) error

	// Subscribe returns a channel of Events. For every published Event evt
	// where evt.Name() is one of names, that Event should be received from the
	// returned Events channel. When ctx is canceled, events channel should be
	// closed by the implementing Bus.
	Subscribe(ctx context.Context, names ...string) (<-chan Event, error)
}
