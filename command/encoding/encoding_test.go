package encoding_test

import (
	"bytes"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/encoding"
)

type mockPayload struct{ A string }

func TestRegister(t *testing.T) {
	encoding.Register("foo", func() command.Payload {
		return mockPayload{}
	})

	var buf bytes.Buffer
	want := mockPayload{A: "foo"}
	if err := encoding.DefaultRegistry.Encode(&buf, "foo", want); err != nil {
		t.Fatalf("DefaultRegistry.Encode shouldn't fail; failed with %q", err)
	}

	load, err := encoding.DefaultRegistry.Decode("foo", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DefaultRegistry.Decode shouldn't fail; failed with %q", err)
	}

	if load != want {
		t.Errorf("DefaultRegistry.Decode should return %v; got %v", want, load)
	}
}
