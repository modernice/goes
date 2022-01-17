package codec_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modernice/goes/codec"
)

func TestJSONRegistry(t *testing.T) {
	reg := codec.JSON(codec.New())

	reg.JSONRegister("foo", func() any { return mockDataA{} })

	var buf bytes.Buffer
	val := "test-val"
	want := mockDataA{A: val}
	if err := reg.Encode(&buf, "foo", want); err != nil {
		t.Fatalf("Encode() failed with %q", err)
	}

	got := buf.String()
	r := bytes.NewReader([]byte(got))

	decoded, err := reg.Decode(r, "foo")
	if err != nil {
		t.Fatalf("Decode() failed with %q", err)
	}

	if decoded.(mockDataA) != want {
		t.Fatalf("decoded data should be %v; is %v\n%s", want, decoded, cmp.Diff(want, decoded))
	}
}
