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
	enc.Register("x-foo", func() event.Data {
		return eventDataA{}
	})

	var encoded bytes.Buffer
	data := eventDataA{
		A: "bar",
		B: moreEventData{A: true},
	}

	err := enc.Encode(&encoded, "x-foo", data)
	if err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	decoded, err := enc.Decode("x-foo", &encoded)
	if err != nil {
		t.Fatal(fmt.Errorf("decode: %w", err))
	}

	if !reflect.DeepEqual(decoded, data) {
		t.Fatal(fmt.Errorf("decoded event data does not match original data\noriginal: %v\ndecoded: %v", data, decoded))
	}
}

func TestGobEncoder_New(t *testing.T) {
	enc := encoding.NewGobEncoder()

	data, err := enc.New("x-foo")
	if !errors.Is(err, encoding.ErrUnregisteredEvent) {
		t.Fatal(fmt.Errorf("expected encoding.ErrUnregisteredEvent error; got %#v", err))
	}

	if data != nil {
		t.Fatal(fmt.Errorf("data should be nil; got %#v", data))
	}

	enc.Register("x-foo", func() event.Data {
		return eventDataA{A: "x-foo"}
	})

	data, err = enc.New("x-foo")
	if err != nil {
		t.Fatal(fmt.Errorf("expected err to be nil; got %#v", err))
	}

	want := eventDataA{A: "x-foo"}
	if data != want {
		t.Fatal(fmt.Errorf("expected event data to equal %#v; got %#v", want, data))
	}
}

func TestGobEncoder_Decode_unregistered(t *testing.T) {
	enc := encoding.NewGobEncoder()

	var buf bytes.Buffer
	enc.Register("x-foo", func() event.Data {
		return eventDataA{}
	})
	err := enc.Encode(&buf, "x-foo", eventDataA{})

	if err != nil {
		t.Fatal(fmt.Errorf("encode: %w", err))
	}

	dec := encoding.NewGobEncoder()

	if _, err = dec.Decode("x-foo", &buf); !errors.Is(err, encoding.ErrUnregisteredEvent) {
		t.Fatal(fmt.Errorf("expected encoding.ErrUnregisterdEvent; got %v", err))
	}
}
