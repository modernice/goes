package event

import "github.com/modernice/goes/codec"

// Encoding is a codec.Encoding with its type parameter set to `any`.
type Encoding = codec.Encoding[any]

// NewRegistry returns a new event registry that can register events with any
// event data.
func NewRegistry() *codec.RegistryOf[any] {
	return codec.New()
}
