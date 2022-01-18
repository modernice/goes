package event

import "github.com/modernice/goes/codec"

type Encoding = codec.Encoding

// NewRegistry returns a new event registry.
func NewRegistry() *codec.Registry {
	return codec.New()
}
