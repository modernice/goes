package codec

import "io"

type Encoding[T any] interface {
	// Encode encodes the given data using the configured encoder for the given name.
	Encode(w io.Writer, name string, data T) error

	// Decode decodes the data in r using the configured decoder for the given name.
	Decode(r io.Reader, name string) (T, error)
}

// Encoder is an encoder for a specific event data or command payload.
type Encoder[T any] interface {
	// Encode encodes the given data and writes the result into w.
	Encode(w io.Writer, data T) error
}

// EncoderFunc allows a function to be used as an Encoder.
type EncoderFunc[T any] func(w io.Writer, data T) error

// Decoder is a decoder for a specific event data or command payload.
type Decoder[T any] interface {
	// Decode decodes the data in r and returns the decoded data.
	Decode(r io.Reader) (T, error)
}

// DecoderFunc allows a function to be used as a Decoder.
type DecoderFunc[T any] func(r io.Reader) (T, error)

// Encode returns encode(w, data).
func (encode EncoderFunc[T]) Encode(w io.Writer, data T) error {
	return encode(w, data)
}

// Decode returns decode(r).
func (decode DecoderFunc[T]) Decode(r io.Reader) (T, error) {
	return decode(r)
}
