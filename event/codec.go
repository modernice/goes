package event

import "github.com/modernice/goes/codec"

// NewRegistry returns a new event registry for encoding and decoding of event
// data for transmission over a network.
//
// Example using encoding/gob for encoding and decoding:
//
//	type FooData struct { ... }
//	type BarData struct { ... }
//
//	reg := codec.Gob(event.NewRegistry())
//	codec.GobRegister[FooData](reg, "foo")
//	codec.GobRegister[BarData](reg, "bar")
//	codec.GobRegister[int](reg, "baz")
//	codec.GobRegister[string](reg, "foobar")
func NewRegistry() *codec.Registry {
	return codec.New()
}
