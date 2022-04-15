package command

import "github.com/modernice/goes/codec"

// NewRegistry returns a new command registry for encoding and decoding of
// command payloads for transmission over a network.
//
// Example using encoding/gob for encoding and decoding:
//
//	type FooPayload struct { ... }
//	type BarPayload struct { ... }
//
//	reg := codec.Gob(command.NewRegistry())
//	codec.GobRegister[FooPayload](reg, "foo")
//	codec.GobRegister[BarPayload](reg, "bar")
//	codec.GobRegister[int](reg, "baz")
//	codec.GobRegister[string](reg, "foobar")
func NewRegistry() *codec.Registry {
	return codec.New()
}
