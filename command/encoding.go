package command

import "github.com/modernice/goes/codec"

// Encoding is a codec.Encoding with its type paramter constrained to `any`.
type Encoding = codec.Encoding[any]

// NewRegistry returns a new command registry that can register commands with
// any command payload.
func NewRegistry() *codec.Registry[any] {
	return codec.New()
}
