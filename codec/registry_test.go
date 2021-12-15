package codec_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/codec"
)

func TestRegistry_Encode_ErrNotFound(t *testing.T) {
	reg := codec.New()

	var w bytes.Buffer
	if err := reg.Encode(&w, "foo", mockDataA{}); !errors.Is(err, codec.ErrNotFound) {
		t.Fatalf("Encode() should fail with %q when data is not registered; got %v", codec.ErrNotFound, err)
	}
}

func TestRegistry_Encode_Decode(t *testing.T) {
	reg := codec.New()

	reg.Register(
		"foo",
		// Register an Encoder that purely uses the mockDataA.A field to encode
		// the data.
		codec.EncoderFunc(func(w io.Writer, data interface{}) error {
			_, err := w.Write([]byte(data.(mockDataA).A))
			return err
		}),
		// Register a Decoder that uses the single string to re-create the data.
		codec.DecoderFunc(func(r io.Reader) (interface{}, error) {
			b, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			return mockDataA{A: string(b)}, nil
		}),
		nil,
	)

	var buf bytes.Buffer
	val := "the-a-value"
	data := mockDataA{A: val}
	if err := reg.Encode(&buf, "foo", data); err != nil {
		t.Fatalf("Encode() failed with %q", err)
	}

	got := buf.String()
	if got != val {
		t.Fatalf("string form of encoded value should be %q; is %q", val, got)
	}

	r := bytes.NewReader([]byte(val))

	decoded, err := reg.Decode(r, "foo")
	if err != nil {
		t.Fatalf("Decode() failed with %q", err)
	}

	if decoded.(mockDataA) != data {
		t.Fatalf("decoded data should be %v; is %v\n%s", data, decoded, cmp.Diff(data, decoded))
	}
}

func TestRegistry_New_ErrMissingFactory(t *testing.T) {
	reg := codec.New()

	if _, err := reg.New("foo"); !errors.Is(err, codec.ErrMissingFactory) {
		t.Fatalf("New() should fail with %q for data that has no factory function; got %v", codec.ErrMissingFactory, err)
	}
}

func TestRegistry_New(t *testing.T) {
	reg := codec.Gob(codec.New())

	var want mockDataA
	reg.GobRegister("foo", func() interface{} { return want })

	data, err := reg.New("foo")
	if err != nil {
		t.Fatalf("New() failed with %q", err)
	}

	if data.(mockDataA) != want {
		t.Fatalf("New() should return %v; got %v\n%s", want, data, cmp.Diff(want, data))
	}
}

type mockDataA struct {
	A string
}
