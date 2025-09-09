package event

import "github.com/modernice/goes/codec"

// NewRegistry creates an empty registry for encoding and decoding event data.
func NewRegistry(opts ...codec.Option) *codec.Registry {
	return codec.New(opts...)
}
