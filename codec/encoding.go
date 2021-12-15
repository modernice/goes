package codec

import "io"

type Encoding interface {
	// Encode encodes the given data using the configured encoder for the given name.
	Encode(w io.Writer, name string, data interface{}) error

	// Decode decodes the data in r using the configured decoder for the given name.
	Decode(r io.Reader, name string) (interface{}, error)
}

// Encoder is an encoder for a specific event data or command payload.
type Encoder interface {
	// Encode encodes the given data and writes the result into w.
	Encode(w io.Writer, data interface{}) error
}

// EncoderFunc allows a function to be used as an Encoder.
type EncoderFunc func(w io.Writer, data interface{}) error

// Decoder is a decoder for a specific event data or command payload.
type Decoder interface {
	// Decode decodes the data in r and returns the decoded data.
	Decode(r io.Reader) (interface{}, error)
}

// DecoderFunc allows a function to be used as a Decoder.
type DecoderFunc func(r io.Reader) (interface{}, error)

// Encode returns encode(w, data).
func (encode EncoderFunc) Encode(w io.Writer, data interface{}) error {
	return encode(w, data)
}

// Decode returns decode(r).
func (decode DecoderFunc) Decode(r io.Reader) (interface{}, error) {
	return decode(r)
}
