package event_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/modernice/goes/event"
)

type mockEventData struct {
	A string
}

type marshalerData struct {
	A string
}

func TestRegistry_Encode(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo", func() event.Data { return mockEventData{} })

	data := mockEventData{A: "test"}

	var buf bytes.Buffer
	if err := reg.Encode(&buf, "mock-foo", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	var decoded event.Data
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("failed to gob decode Data: %v", err)
	}

	if decoded != data {
		t.Fatalf("decoded differs from encoded Data. want=%#v got=%#v", data, decoded)
	}
}

func TestRegistry_New_unregistered(t *testing.T) {
	reg := event.NewRegistry()
	if _, err := reg.New("mock-foo"); !errors.Is(err, event.ErrUnregistered) {
		t.Fatalf("New with an unregistered Event name should return %q; got %q", event.ErrUnregistered, err)
	}
}

func TestRegistry_New(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo", func() event.Data { return mockEventData{} })

	data, err := reg.New("mock-foo")
	if err != nil {
		t.Fatalf("New shouldn't fail with a registered Event name; got %q", err)
	}

	want := mockEventData{}
	if data != want {
		t.Fatalf("New should return %#v; got %#v", want, data)
	}
}

func TestRegistry_Decode_unregistered(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo", func() event.Data { return mockEventData{} })

	data := mockEventData{}
	var buf bytes.Buffer
	if err := reg.Encode(&buf, "mock-foo", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	reg = event.NewRegistry()
	if _, err := reg.Decode("mock-foo", &buf); !errors.Is(err, event.ErrUnregistered) {
		t.Fatalf("Decode should fail with %q; got %q", event.ErrUnregistered, err)
	}
}

func TestRegistry_Decode(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo", func() event.Data { return mockEventData{} })

	data := mockEventData{A: "test"}
	var buf bytes.Buffer
	if err := reg.Encode(&buf, "mock-foo", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	decoded, err := reg.Decode("mock-foo", &buf)
	if err != nil {
		t.Fatalf("Decode shouldn't fail; failed with %q", err)
	}

	if decoded != data {
		t.Fatalf("decoded differs from encoded Data. want=%v got=%v", data, decoded)
	}
}

func TestGobNameFunc(t *testing.T) {
	type mockData struct{}

	reg := event.NewRegistry(event.GobNameFunc(func(name string) string {
		return fmt.Sprintf("test_%s", name)
	}))
	reg.Register("mock-a", func() event.Data { return mockData{} })

	type otherData struct{}

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("gob.RegisterName(%q) should panic because that name should already be registered for %T", "test_mock-a", mockData{})
		}
	}()

	gob.RegisterName("test_mock-a", otherData{})
}

func TestGobNameFunc_empty(t *testing.T) {
	type mockData struct{}

	reg := event.NewRegistry(event.GobNameFunc(func(string) string { return "" }))
	reg.Register("mock-a", func() event.Data { return mockData{} })

	type otherData struct{}

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("gob.RegisterName(%q) should panic because that name should already be registered for %T", defaultGobName(otherData{}), mockData{})
		}
	}()

	gob.RegisterName(defaultGobName(otherData{}), mockData{})
}

func TestRegistry_Encode_marshaler(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo-marshaler", func() event.Data { return marshalerData{} })

	var buf bytes.Buffer
	data := marshalerData{A: "test"}
	if err := reg.Encode(&buf, "mock-foo-marshaler", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}
}

func TestRegistry_Decode_marshaler(t *testing.T) {
	reg := event.NewRegistry()
	reg.Register("mock-foo-marshaler", func() event.Data { return marshalerData{} })

	var buf bytes.Buffer
	data := marshalerData{A: "test"}
	if err := reg.Encode(&buf, "mock-foo-marshaler", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}
	b := buf.Bytes()

	var unmarshaled marshalerData
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal shouldn't fail; failed with %q", err)
	}

	if unmarshaled != data {
		t.Fatalf("unmarshaled differs from encoded Data. want=%#v got=%#v", data, unmarshaled)
	}

	decoded, err := reg.Decode("mock-foo-marshaler", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Decode shouldn't fail; failed with %q", err)
	}

	if decoded != data {
		t.Fatalf("decoded differs from encoded Data. want=%#v got=%#v", data, decoded)
	}
}

func (d marshalerData) MarshalEvent() ([]byte, error) {
	return json.Marshal(d)
}

func (d *marshalerData) UnmarshalEvent(data []byte) error {
	return json.Unmarshal(data, d)
}

// copied from encoding/gob
func defaultGobName(v interface{}) string {
	rt := reflect.TypeOf(v)
	name := rt.String()
	star := ""
	if rt.Name() == "" {
		if pt := rt; pt.Kind() == reflect.Ptr {
			star = "*"
			rt = pt
		}
	}
	if rt.Name() != "" {
		if rt.PkgPath() == "" {
			name = star + rt.Name()
		} else {
			name = star + rt.PkgPath() + "." + rt.Name()
		}
	}
	return name
}
