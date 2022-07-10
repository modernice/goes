package command

import "github.com/modernice/goes/codec"

// NewRegistry returns a new command registry for encoding and decoding of
// command payloads for transmission over a network.
func NewRegistry(opts ...codec.Option) *codec.Registry {
	return codec.New(opts...)
}
