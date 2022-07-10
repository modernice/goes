package event

import "github.com/modernice/goes/codec"

// NewRegistry returns a new event registry for encoding and decoding of event
// data for transmission over a network.
func NewRegistry(opts ...codec.Option) *codec.Registry {
	return codec.New(opts...)
}
