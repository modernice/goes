package eventstoreui

import (
	"errors"
	"reflect"
	"testing"
)

type testDecoder struct {
	name string
}

func (decoder *testDecoder) Unmarshal(raw []byte, name string) (any, error) {
	decoder.name = name
	if string(raw) == "bad" {
		return nil, errors.New("custom decode failed")
	}
	return map[string]any{"decoded": string(raw)}, nil
}

func TestDecodeEventDataUsesConfiguredDecoder(t *testing.T) {
	decoder := new(testDecoder)
	data, message := decodeEventData(decoder, "order.created", []byte("wire-payload"))
	if message != "" {
		t.Fatalf("unexpected decode error: %s", message)
	}
	if decoder.name != "order.created" {
		t.Fatalf("expected event name to reach decoder; got %q", decoder.name)
	}
	want := map[string]any{"decoded": "wire-payload"}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("unexpected decoded data: %#v", data)
	}
}

func TestDecodeEventDataKeepsPerEventError(t *testing.T) {
	data, message := decodeEventData(new(testDecoder), "order.created", []byte("bad"))
	if data != nil || message != "custom decode failed" {
		t.Fatalf("unexpected decode result: data=%#v message=%q", data, message)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	want := eventCursor{TimeNano: 12345}
	var got eventCursor
	if err := decodeCursor(encodeCursor(want), &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("cursor mismatch: got %#v want %#v", got, want)
	}
}
