package command_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/modernice/goes/command"
)

type mockRegistryPayload struct {
	A string
}

type marshalerPayload struct {
	A string
}

func TestRegistry_Encode(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("foo", func() command.Payload { return mockRegistryPayload{} })

	load := mockRegistryPayload{A: "test"}

	var buf bytes.Buffer
	if err := reg.Encode(&buf, "foo", load); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	var decoded command.Payload
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("failed to gob decode Payload: %v", err)
	}

	if decoded != load {
		t.Fatalf("decoded differs from encoded Payload. want=%#v got=%#v", load, decoded)
	}
}

func TestRegistry_New_unregistered(t *testing.T) {
	reg := command.NewRegistry()
	if _, err := reg.New("mock-foo"); !errors.Is(err, command.ErrUnregistered) {
		t.Fatalf("New with an unregistered Command name should return %q; got %q", command.ErrUnregistered, err)
	}
}

func TestRegistry_New(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("foo", func() command.Payload { return mockRegistryPayload{} })

	load, err := reg.New("foo")
	if err != nil {
		t.Fatalf("New shouldn't fail with a registered Command name; got %q", err)
	}

	want := mockRegistryPayload{}
	if load != want {
		t.Fatalf("New should return %#v; got %#v", want, load)
	}
}

func TestRegistry_Decode_unregistered(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("foo", func() command.Payload { return mockRegistryPayload{} })

	load := mockRegistryPayload{}
	var buf bytes.Buffer
	if err := reg.Encode(&buf, "foo", load); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	reg = command.NewRegistry()
	if _, err := reg.Decode("foo", &buf); !errors.Is(err, command.ErrUnregistered) {
		t.Fatalf("Decode should fail with %q; got %q", command.ErrUnregistered, err)
	}
}

func TestRegistry_Decode(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("foo", func() command.Payload { return mockRegistryPayload{} })

	data := mockRegistryPayload{A: "test"}
	var buf bytes.Buffer
	if err := reg.Encode(&buf, "foo", data); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}

	decoded, err := reg.Decode("foo", &buf)
	if err != nil {
		t.Fatalf("Decode shouldn't fail; failed with %q", err)
	}

	if decoded != data {
		t.Fatalf("decoded differs from encoded Payload. want=%v got=%v", data, decoded)
	}
}

func TestGobNameFunc(t *testing.T) {
	type mockPayload struct{}

	reg := command.NewRegistry(command.GobNameFunc(func(name string) string {
		return fmt.Sprintf("test_%s", name)
	}))
	reg.Register("mock-a", func() command.Payload { return mockPayload{} })

	type otherData struct{}

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("gob.RegisterName(%q) should panic because that name should already be registered for %T", "test_mock-a", mockPayload{})
		}
	}()

	gob.RegisterName("test_mock-a", otherData{})
}

func TestGobNameFunc_empty(t *testing.T) {
	type mockPayload struct{}

	reg := command.NewRegistry(command.GobNameFunc(func(string) string { return "" }))
	reg.Register("mock-a", func() command.Payload { return mockPayload{} })

	type otherPayload struct{}

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("gob.RegisterName(%q) should panic because that name should already be registered for %T", defaultGobName(otherPayload{}), mockPayload{})
		}
	}()

	gob.RegisterName(defaultGobName(otherPayload{}), mockPayload{})
}

func TestRegistry_Encode_marshaler(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("mock-foo-marshaler", func() command.Payload { return marshalerPayload{} })

	var buf bytes.Buffer
	load := marshalerPayload{A: "test"}
	if err := reg.Encode(&buf, "mock-foo-marshaler", load); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}
}

func TestRegistry_Decode_marshaler(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("mock-foo-marshaler", func() command.Payload { return marshalerPayload{} })

	var buf bytes.Buffer
	load := marshalerPayload{A: "test"}
	if err := reg.Encode(&buf, "mock-foo-marshaler", load); err != nil {
		t.Fatalf("Encode shouldn't fail; failed with %q", err)
	}
	b := buf.Bytes()

	var unmarshaled marshalerPayload
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal shouldn't fail; failed with %q", err)
	}

	if unmarshaled != load {
		t.Fatalf("unmarshaled differs from encoded Data. want=%#v got=%#v", load, unmarshaled)
	}

	decoded, err := reg.Decode("mock-foo-marshaler", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Decode shouldn't fail; failed with %q", err)
	}

	if decoded != load {
		t.Fatalf("decoded differs from encoded Payload. want=%#v got=%#v", load, decoded)
	}
}

func (d marshalerPayload) MarshalCommand() ([]byte, error) {
	return json.Marshal(d)
}

func (d *marshalerPayload) UnmarshalCommand(data []byte) error {
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
