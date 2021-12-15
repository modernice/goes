package codec

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	// ErrNotFound is returned when trying to encode/decode data which hasn't
	// been registered into a registry.
	ErrNotFound = errors.New("encoding not found. forgot to register?")

	// ErrMissingFactory is returned when trying to instantiate data for which
	// no factory function was provided.
	ErrMissingFactory = errors.New("missing factory for data. forgot to register?")
)

// A Registry provides the Encoders and Decoders for event data or command
// payloads. Use the Register method to register the Encoder and Decoder for a
// specific type.
//
// You likely don't want to use this registry directly, as it requires you to
// define an Encoder and Decoder for every registered type/name. You can for
// example wrap this *Registry in a *GobRegistry to use encoding/gob for
// encoding and decoding data:
//
// Register
//
//	type fooData struct { ... }
//	reg := Gob(New())
//	reg.GobRegister("foo", func() interface{} { return fooData{}})
//
// Encode
//
//	var w io.Writer
//	err := reg.Encode(w, "foo", someData{...})
//
// Decode
//
//	var r io.Reader
//	err := reg.Decode(r, "foo")
type Registry struct {
	sync.RWMutex

	encoders  map[string]Encoder
	decoders  map[string]Decoder
	factories map[string]func() interface{}
}

// New returns a new Registry for event data or command payloads.
func New() *Registry {
	return &Registry{
		encoders:  make(map[string]Encoder),
		decoders:  make(map[string]Decoder),
		factories: make(map[string]func() interface{}),
	}
}

// Register registers the given Encoder and Decoder under the given name.
// When reg.Encode is called, the provided Encoder is be used to encode the
// given data. When reg.Decode is called, the provided Decoder is used. The
// provided makeFunc is optional and only used to create data when calling
// reg.New().
func (reg *Registry) Register(name string, enc Encoder, dec Decoder, makeFunc func() interface{}) {
	reg.Lock()
	defer reg.Unlock()
	reg.encoders[name] = enc
	reg.decoders[name] = dec
	reg.factories[name] = makeFunc
}

// Encode encodes the data that is registered under the given name using the
// registered Encoder. If no Encoder is registered for the given name, an error
// that unwraps to ErrNotFound is returned.
func (reg *Registry) Encode(w io.Writer, name string, data interface{}) error {
	reg.RLock()
	defer reg.RUnlock()

	if enc, ok := reg.encoders[name]; ok {
		return enc.Encode(w, data)
	}

	return fmt.Errorf("get encoder: %w [name=%v]", ErrNotFound, name)
}

// Decode decodes the data that is registered under the given name using the
// registered Decoder. If no Decoder is registered for the give name, an error
// that unwraps to ErrNotFound is returned.
func (reg *Registry) Decode(r io.Reader, name string) (interface{}, error) {
	reg.RLock()
	defer reg.RUnlock()

	if dec, ok := reg.decoders[name]; ok {
		return dec.Decode(r)
	}

	return nil, fmt.Errorf("get decoder: %w [name=%v]", ErrNotFound, name)
}

// New creates and returns a new instance of the data that is registered under
// the given name. If no factory function was provided for this data,
// ErrMissingFactory is returned.
func (reg *Registry) New(name string) (interface{}, error) {
	reg.RLock()
	defer reg.RUnlock()

	if makeFunc, ok := reg.factories[name]; ok {
		return makeFunc(), nil
	}

	return nil, ErrMissingFactory
}
