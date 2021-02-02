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
	enc.Register("foo", mockPayloadA{})

	var encoded bytes.Buffer
	pl := mockPayloadA{
		A: true,
		B: "bar",
	}

	err := enc.Encode(&encoded, pl)
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

	enc.Register("foo", mockPayloadA{})

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

func TestGobEncoder_New_pointer(t *testing.T) {
	enc := encoding.NewGobEncoder()
	give := &mockPayloadB{B: true, A: "bar"}
	enc.Register("bar", give)

	pl, err := enc.New("bar")
	if err != nil {
		t.Fatalf("enc.New should return no error; got %#v", err)
	}

	if pl == nil {
		t.Fatalf("enc.New should return some payload; git %#v", pl)
	}

	if pl == give {
		t.Fatalf(
			"payload returned by enc.New points to the same address as original"+
				" data.\n\noriginal: %p\n\nnew: %p",
			&give,
			&pl,
		)
	}
}

func TestGobEncoder_Decode_unregistered(t *testing.T) {
	enc := encoding.NewGobEncoder()

	var buf bytes.Buffer
	enc.Register("foo", mockPayloadA{})
	err := enc.Encode(&buf, mockPayloadA{})

	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := encoding.NewGobEncoder()

	if _, err = dec.Decode("foo", &buf); !errors.Is(err, encoding.ErrUnregisteredCommand) {
		t.Fatalf("encoder should return encoding.ErrUnregisteredCommand; got %v", err)
	}
}

func TestGobEncoder_Register_pointer(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.Register("bar", &mockPayloadB{})

	var buf bytes.Buffer
	pl := &mockPayloadB{A: "foo", B: true}
	err := enc.Encode(&buf, &pl)
	if err != nil {
		t.Fatalf("encode %v: %v", pl, err)
	}

	decoded, err := enc.Decode("bar", &buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	decodedType := reflect.TypeOf(decoded)
	if decodedType.Kind() != reflect.Ptr {
		t.Fatalf("decoded payload should be of type %T; got %T", pl, decoded)
	}

	if !reflect.DeepEqual(decoded, pl) {
		t.Fatalf(
			"decoded payload does not match original data.\n\n"+
				"original: %#v\n\ndecoded: %#v\n\n",
			pl,
			decoded,
		)
	}
}

func TestGobEncoder_Register_forceZeroValue(t *testing.T) {
	enc := encoding.NewGobEncoder()
	give := mockPayloadA{A: true, B: "foo"}
	enc.Register("foo", give)

	pl, err := enc.New("foo")
	if err != nil {
		t.Fatalf("enc.New(%q): %v", "foo", err)
	}

	want := mockPayloadA{}
	if pl != want {
		t.Fatalf("enc.New should return zero-value payload; got %#v", pl)
	}
}

func TestGobEncoder_RegisterMany(t *testing.T) {
	enc := encoding.NewGobEncoder()
	enc.RegisterMany(map[string]command.Payload{
		"foo": mockPayloadA{},
		"bar": mockPayloadB{},
	})

	var buf bytes.Buffer
	var err error
	if err = enc.Encode(&buf, mockPayloadA{}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err = enc.Decode("foo", &buf); err != nil {
		t.Fatalf("decode %q: %v", "foo", err)
	}

	buf = bytes.Buffer{}
	if err = enc.Encode(&buf, mockPayloadB{}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err = enc.Decode("bar", &buf); err != nil {
		t.Fatalf("decode %q: %v", "bar", err)
	}

	buf = bytes.Buffer{}
	if err = enc.Encode(&buf, mockPayloadC{}); err == nil {
		t.Fatalf("expected a gob error; got %#v", err)
	}
}
