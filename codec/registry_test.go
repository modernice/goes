package codec_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

// FooData represents a type that contains a string and an integer, used in
// testing for encoding and decoding data using different codecs.
type FooData struct {
	Foo string
	Bar int
}

// BarData is a struct type used to encode and decode binary data for events. It
// implements methods to Marshal and Unmarshal data, and is registered with a
// codec.Registry during tests.
type BarData struct {
	Foo string
	Bar int
}

// Marshal returns the gob-encoded byte slice representation of the BarData
// value.
func (data BarData) Marshal() ([]byte, error) {
	var out bytes.Buffer
	if err := gob.NewEncoder(&out).Encode(data); err != nil {
		return nil, fmt.Errorf("gob encode data: %v", err)
	}
	return out.Bytes(), nil
}

// Unmarshal decodes a byte slice into a BarData value.
func (data *BarData) Unmarshal(b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(data)
}

func TestRegistry_Marshal_Unmarshal_default(t *testing.T) {
	r := codec.New()
	codec.Register[FooData](r, "foo")

	evt := event.New("foo", FooData{"hello", 123})

	b, err := r.Marshal(evt.Data())
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	jsonb, err := json.Marshal(evt.Data())
	if err != nil {
		t.Fatalf("failed to marshal event data as json: %v", err)
	}

	if !bytes.Equal(b, jsonb) {
		t.Fatalf("marshaled event data does not match json data.\n%s", cmp.Diff(jsonb, b))
	}

	decoded, err := r.Unmarshal(b, "foo")
	if err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}

	decodedt, ok := decoded.(FooData)
	if !ok {
		t.Fatalf("decoded event data is not of type %T", decodedt)
	}

	if decodedt != evt.Data() {
		t.Fatalf("decoded event data does not match original event data.\n%s", cmp.Diff(decodedt, evt.Data()))
	}
}

func TestRegistry_Marshal_Unmarshal_custom(t *testing.T) {
	r := codec.New()
	codec.Register[BarData](r, "bar")

	evt := event.New("bar", BarData{"hello", 123})

	b, err := r.Marshal(evt.Data())
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var encoded bytes.Buffer
	if err := gob.NewEncoder(&encoded).Encode(evt.Data()); err != nil {
		t.Fatalf("failed to marshal event data as gob: %v", err)
	}

	gobb := encoded.Bytes()

	if !bytes.Equal(b, gobb) {
		t.Fatalf("marshaled event data does not match gob data.\n%s", cmp.Diff(gobb, b))
	}

	decoded, err := r.Unmarshal(b, "bar")
	if err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}

	decodedt, ok := decoded.(BarData)
	if !ok {
		t.Fatalf("decoded event data is not of type %T", decodedt)
	}

	if decodedt != evt.Data() {
		t.Fatalf("decoded event data does not match original event data.\n%s", cmp.Diff(decodedt, evt.Data()))
	}
}

func TestRegistry_New(t *testing.T) {
	r := codec.New()
	codec.Register[FooData](r, "foo")

	d, err := r.New("foo")
	if err != nil {
		t.Fatalf("failed to create new data: %v", err)
	}

	td, ok := d.(*FooData)
	if !ok {
		t.Fatalf("created data is not of type %T", td)
	}

	var want FooData
	if *td != want {
		t.Fatalf("created data should be zero value %v, got %v", want, td)
	}
}

func TestDefault(t *testing.T) {
	r := codec.New(codec.Default(
		func(data any) ([]byte, error) {
			var buf bytes.Buffer
			err := gob.NewEncoder(&buf).Encode(data)
			return buf.Bytes(), err
		},
		func(b []byte, data any) error {
			return gob.NewDecoder(bytes.NewReader(b)).Decode(data)
		},
	))

	codec.Register[FooData](r, "foo")

	evt := event.New("foo", FooData{"hello", 123})

	b, err := r.Marshal(evt.Data())
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var encoded bytes.Buffer
	if err := gob.NewEncoder(&encoded).Encode(evt.Data()); err != nil {
		t.Fatalf("failed to marshal event data as gob: %v", err)
	}

	gobb := encoded.Bytes()
	if !bytes.Equal(gobb, b) {
		t.Fatalf("marshaled event data does not match gob data.\n%s", cmp.Diff(gobb, b))
	}

	decoded, err := r.Unmarshal(b, "foo")
	if err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}

	decodedt, ok := decoded.(FooData)
	if !ok {
		t.Fatalf("decoded event data is not of type %T", decodedt)
	}

	if decodedt != evt.Data() {
		t.Fatalf("decoded event data does not match original event data.\n%s", cmp.Diff(evt.Data(), decodedt))
	}
}

func TestMake(t *testing.T) {
	r := codec.New()
	codec.Register[FooData](r, "foo")

	d, err := codec.Make[FooData](r, "foo")
	if err != nil {
		t.Fatalf("failed to make %T data: %v", d, err)
	}

	var want FooData
	if d != want {
		t.Fatalf("created data should be zero value %v, got %v", want, d)
	}
}
