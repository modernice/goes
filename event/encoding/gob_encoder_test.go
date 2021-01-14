package encoding_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/encoding"
)

var _ event.Encoder = &encoding.GobEncoder{}

type eventDataA struct {
	A string
	B moreEventData
}

type eventDataB struct {
	A moreEventData
	B string
}

type eventDataC struct {
	A string
	B moreEventData
}

type moreEventData struct {
	A bool
}

func TestGobEncoder(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", eventDataA{})

	var encoded bytes.Buffer
	data := eventDataA{
		A: "bar",
		B: moreEventData{A: true},
	}

	err := enc.Encode(&encoded, data)
	if err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	decoded, err := enc.Decode("foo", &encoded)
	if err != nil {
		t.Fatal(fmt.Errorf("decode: %w", err))
	}

	if !reflect.DeepEqual(decoded, data) {
		t.Fatal(fmt.Errorf("decoded event data does not match original data\noriginal: %v\ndecoded: %v", data, decoded))
	}
}

func TestGobEncoder_Decode_unregistered(t *testing.T) {
	enc := encoding.NewGobEncoder()

	var buf bytes.Buffer
	enc.Register("foo", eventDataA{})
	err := enc.Encode(&buf, eventDataA{})

	if err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	dec := encoding.NewGobEncoder()

	if _, err = dec.Decode("foo", &buf); !errors.Is(err, encoding.ErrUnregisteredEvent) {
		t.Fatal(fmt.Errorf("expected encoding.ErrUnregisterdEvent; got %v", err))
	}
}

func TestGobEncoder_Register_pointer(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("bar", &eventDataB{})

	var buf bytes.Buffer
	data := &eventDataB{A: moreEventData{A: true}, B: "baz"}
	err := enc.Encode(&buf, data)
	if err != nil {
		t.Fatal(fmt.Errorf("encode %v: %w", data, err))
	}

	decoded, err := enc.Decode("bar", &buf)
	if err != nil {
		t.Fatal(fmt.Errorf("docode: %w", err))
	}

	decodedType := reflect.TypeOf(decoded)
	if decodedType.Kind() != reflect.Ptr {
		t.Fatal(fmt.Errorf("decoded data should be of type %T; got %T", data, decoded))
	}

	if !reflect.DeepEqual(decoded, data) {
		t.Fatal(fmt.Errorf("decoded data does not match original data\noriginal: %v\ndecoded: %v", data, decoded))
	}

	if decoded == data {
		t.Fatal("decoded data points to original data")
	}
}

func TestGobEncoder_RegisterMany(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.RegisterMany(map[string]event.Data{
		"foo": eventDataA{},
		"bar": eventDataB{},
	})

	var buf bytes.Buffer
	var err error
	if err = enc.Encode(&buf, eventDataA{}); err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	if _, err := enc.Decode("foo", &buf); err != nil {
		t.Fatal(fmt.Errorf(`decode "foo": %w`, err))
	}

	buf = bytes.Buffer{}
	if err = enc.Encode(&buf, eventDataB{}); err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	if _, err := enc.Decode("bar", &buf); err != nil {
		t.Fatal(fmt.Errorf(`decode "bar": %w`, err))
	}

	buf = bytes.Buffer{}
	if err = enc.Encode(&buf, eventDataC{}); err == nil {
		t.Fatal(fmt.Errorf(`expected gob error; got %v`, err))
	}
}
