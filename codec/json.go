package codec

import (
	"encoding/json"
	"io"

	"github.com/mitchellh/mapstructure"
)

var jsonEnc jsonEncoder

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

func (reg *JSONRegistry) UseMapstructure(use bool) {
	reg.useMapstructure = use
}

// JSONRegister registers data with the given name into the underlying registry.
// makeFunc is used create instances of the data and encoding/json will be used
// to encode and decode the data returned by makeFunc.
func (reg *JSONRegistry) JSONRegister(name string, makeFunc func() interface{}) {
	if makeFunc == nil {
		panic("[goes/codec.JSONRegistry.JSONRegister] nil makeFunc")
	}

	reg.Registry.Register(name, jsonEnc, jsonDecoder{name: name, makeFunc: makeFunc}, makeFunc)
}

type jsonEncoder struct{}

func (jsonEncoder) Encode(w io.Writer, data interface{}) error {
	return json.NewEncoder(w).Encode(&data)
}

type jsonDecoder struct {
	name            string
	makeFunc        func() interface{}
	useMapstructure bool
}

func (dec jsonDecoder) Decode(r io.Reader) (interface{}, error) {
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
