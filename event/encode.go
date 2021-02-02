package event

//go:generate mockgen -source=encode.go -destination=./mocks/encode.go

import (
	"io"
)

// An Encoder encodes & decodes Data.
type Encoder interface {
	// Encode encodes d and writes the result into w.
	Encode(w io.Writer, d Data) error

	// Decode decodes the Data in r based on the specified Event name.
	Decode(name string, r io.Reader) (Data, error)
}

// A Registry is an Encoder that also allows to register new Event types.
type Registry interface {
	Encoder

	// Register registers a new Event with the given name and Data.
	Register(name string, d Data)
}
