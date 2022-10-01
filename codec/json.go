package codec

import (
	"encoding/json"
	"io"

	"github.com/mitchellh/mapstructure"
)

// A JSONRegistry allows registering data into a Registry using factory
// functions. Data that is registered via a JSONRegistry will be encoded and
// decoded using the encoding/json package.
type JSONRegistry struct {
	*Registry

	useMapstructure bool
}

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

func JSONRegister[T any](r *JSONRegistry, name string) {
	Register[T](
		r.Registry,
		name,
		jsonEncoder[T]{},
		jsonDecoder[T]{name: name, makeFunc: func() (v T) { return v }},
	)
}

func (reg *JSONRegistry) UseMapstructure(use bool) {
	reg.useMapstructure = use
}

// JSONRegister registers data with the given name into the underlying registry.
// makeFunc is used create instances of the data and encoding/json will be used
// to encode and decode the data returned by makeFunc.
func (r *JSONRegistry) JSONRegister(name string, makeFunc func() any) {
	registerWithFactoryFunc[any](
		r.Registry,
		name,
		jsonEncoder[any]{},
		jsonDecoder[any]{name: name, makeFunc: makeFunc},
		makeFunc,
	)
}

type jsonEncoder[T any] struct{}

func (jsonEncoder[T]) Encode(w io.Writer, data T) error {
	return json.NewEncoder(w).Encode(&data)
}

type jsonDecoder[T any] struct {
	name            string
	makeFunc        func() T
	useMapstructure bool
}

func (dec jsonDecoder[T]) Decode(r io.Reader) (T, error) {
	data := dec.makeFunc()

	if !dec.useMapstructure {
		err := json.NewDecoder(r).Decode(&data)
		return data, err
	}

	untyped := make(map[string]interface{})

	if err := json.NewDecoder(r).Decode(&untyped); err != nil {
		return data, err
	}

	if err := mapstructure.Decode(untyped, &data); err != nil {
		return data, err
	}

	return data, nil
}