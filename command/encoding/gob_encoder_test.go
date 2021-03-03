package encoding_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/encoding"
)

var _ = &encoding.GobEncoder{}

type mockPayloadA struct {
	A bool
	B string
}

type mockPayloadB struct {
	A string
	B bool
}

type mockPayloadC struct {
	A int
	B string
}

func TestGobEncoder(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", func() command.Payload {
		return mockPayloadA{}
	})

	var encoded bytes.Buffer
	pl := mockPayloadA{
		A: true,
		B: "bar",
	}

	err := enc.Encode(&encoded, "foo", pl)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := enc.Decode("foo", &encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !reflect.DeepEqual(decoded, pl) {
		t.Fatalf(
			"decoded command payload does not match original.\n\n"+
				"original: %#v\n\ndecoded: %#v\n\n",
			pl,
			decoded,
		)
	}
}

func TestGobEncoder_New(t *testing.T) {
	enc := encoding.NewGobEncoder()

	pl, err := enc.New("foo")
	if !errors.Is(err, encoding.ErrUnregisteredCommand) {
		t.Fatalf(
			"enc.New should return encoding.ErrUnregisteredCommand; got %#v",
			err,
		)
	}

	if pl != nil {
		t.Fatalf("payload should be <nil>; got %#v", pl)
	}

	enc.Register("foo", func() command.Payload {
		return mockPayloadA{}
	})

	pl, err = enc.New("foo")
	if err != nil {
		t.Fatalf("enc.New should return no error; got %#v", err)
	}

	want := mockPayloadA{}
	if pl != want {
		t.Fatalf(
			"enc.New returned wrong payload.\n\nwant: %#v; got: %#v\n\n",
			want,
			pl,
		)
	}
}

func TestGobEncoder_Decode_unregistered(t *testing.T) {
	enc := encoding.NewGobEncoder()

	var buf bytes.Buffer
	enc.Register("foo", func() command.Payload {
		return mockPayloadA{}
	})
	err := enc.Encode(&buf, "foo", mockPayloadA{})

	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := encoding.NewGobEncoder()

	if _, err = dec.Decode("foo", &buf); !errors.Is(err, encoding.ErrUnregisteredCommand) {
		t.Fatalf("encoder should return encoding.ErrUnregisteredCommand; got %v", err)
	}
}
