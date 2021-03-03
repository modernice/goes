package encoding_test

import (
	"bytes"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/encoding"
	"github.com/modernice/goes/event/test"
)

func TestRegister(t *testing.T) {
	encoding.Register("foo", func() event.Data {
		return test.FooEventData{}
	})

	d := test.FooEventData{A: "foo"}
	var buf bytes.Buffer
	if err := encoding.DefaultRegistry.Encode(&buf, "foo", d); err != nil {
		t.Fatalf("DefaultRegistry.Encode shouldn't fail; failed with %q", err)
	}

	data, err := encoding.DefaultRegistry.Decode("foo", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DefaultRegistry.Decode shouldn't fail; failed with %q", err)
	}

	if data != d {
		t.Errorf("DefaultRegistry.Decode should return %#v; got %#v", d, data)
	}
}
