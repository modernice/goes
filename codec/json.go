package codec

import (
	"encoding/json"
	"io"

	"github.com/mitchellh/mapstructure"
)

// A JSONRegistry allows registering data into a Registry using factory
// functions. Data that is registered via a JSONRegistry will be encoded and
// decoded using the encoding/json package.
type JSONRegistry struct{ *Registry }

// JSON wraps the given Registry in a JSONRegistry. The JSONRegistry provides a
// JSONRegister function to register data using a factory function.
//
// If reg is nil, a new underlying Registry is created with New().
func JSON(reg *Registry) *JSONRegistry {
	if reg == nil {
		reg = New()
	}
	return &JSONRegistry{Registry: reg}
}

func JSONRegister[T any](r *JSONRegistry, name string, makeFunc func() T) {
	if makeFunc == nil {
		panic("[goes/codec.JSONRegistry.JSONRegister] nil makeFunc")
	}

	Register[T](
		r.Registry,
		name,
		jsonEncoder[T]{},
		jsonDecoder[T]{name: name, makeFunc: makeFunc},
		makeFunc,
	)
}

// JSONRegister registers data with the given name into the underlying registry.
// makeFunc is used create instances of the data and encoding/json will be used
// to encode and decode the data returned by makeFunc.
func (reg *JSONRegistry) JSONRegister(name string, makeFunc func() any) {
	JSONRegister(reg, name, makeFunc)
}

type jsonEncoder[T any] struct{}

func (jsonEncoder[T]) Encode(w io.Writer, data T) error {
	return json.NewEncoder(w).Encode(&data)
}

type jsonDecoder[T any] struct {
	name     string
	makeFunc func() T
}

func (dec jsonDecoder[T]) Decode(r io.Reader) (T, error) {
	data := dec.makeFunc()

	untyped := make(map[string]any)

	if err := json.NewDecoder(r).Decode(&untyped); err != nil {
		return data, err
	}

	if err := mapstructure.Decode(untyped, &data); err != nil {
		return data, err
	}

	return data, nil
}
