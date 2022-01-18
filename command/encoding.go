package command

import "github.com/modernice/goes/codec"

type Encoding = codec.Encoding

// NewRegistry returns a new command registry that can register commands with
// any command payload.
func NewRegistry() *codec.Registry {
	return codec.New()
}
