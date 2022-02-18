package snapshot_test

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
)

type mockAggregate struct {
	*aggregate.Base[uuid.UUID]
	mockState
}

type mockSnapshotter mockAggregate

type mockState struct {
	A bool
	B int
	C string
}

func TestMarshal_default(t *testing.T) {
	a := &mockAggregate{
		Base: aggregate.New("foo", uuid.New()),
		mockState: mockState{
			A: true,
			B: -10,
			C: "foo",
		},
	}

	if _, err := snapshot.Marshal(a); err != snapshot.ErrUnimplemented {
		t.Fatalf("Marshal() should fail with %q; got %v", snapshot.ErrUnimplemented, err)
	}
}

func TestMarshal_marshaler(t *testing.T) {
	a := &mockSnapshotter{
		Base: aggregate.New("foo", uuid.New()),
		mockState: mockState{
			A: true,
			B: -10,
			C: "foo",
		},
	}

	if _, err := snapshot.Marshal(a); err != nil {
		t.Fatalf("Marshal shouldn't fail; failed with %q", err)
	}
}

func TestUnmarshal(t *testing.T) {
	a := &mockAggregate{
		Base: aggregate.New("foo", uuid.New()),
		mockState: mockState{
			A: true,
			B: -10,
			C: "foo",
		},
	}

	b, err := snapshot.Marshal(a)
	if err != snapshot.ErrUnimplemented {
		t.Fatalf("Marshal() should fail with %q; got %v", snapshot.ErrUnimplemented, err)
	}
	snap, _ := snapshot.New[uuid.UUID](a, snapshot.Data(b))

	unmarshaled := &mockAggregate{Base: aggregate.New("foo", uuid.New())}

	if err = snapshot.Unmarshal(snap, unmarshaled); err != snapshot.ErrUnimplemented {
		t.Fatalf("Unmarshal() should fail with %q; got %v", snapshot.ErrUnimplemented, err)
	}

	var want mockState
	if unmarshaled.mockState != want {
		t.Errorf("unmarshaled state should be zero value. want=%v got=%v", want, unmarshaled.mockState)
	}
}

func TestUnmarshal_unmarshaler(t *testing.T) {
	a := &mockSnapshotter{
		Base: aggregate.New("foo", uuid.New()),
		mockState: mockState{
			A: true,
			B: -10,
			C: "foo",
		},
	}

	b, err := snapshot.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal shouldn't fail; failed with %q", err)
	}
	snap, _ := snapshot.New[uuid.UUID](a, snapshot.Data(b))

	unmarshaled := &mockSnapshotter{Base: aggregate.New("foo", uuid.New())}

	if err = snapshot.Unmarshal(snap, unmarshaled); err != nil {
		t.Fatalf("Unmarshal shouldn't fail; failed with %q", err)
	}

	if unmarshaled.mockState != a.mockState {
		t.Errorf("unmarshaled state differs from original. want=%v got=%v", a.mockState, unmarshaled.mockState)
	}
}

func (a *mockSnapshotter) MarshalSnapshot() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(a.mockState); err != nil {
		return nil, fmt.Errorf("gob: %w", err)
	}
	return buf.Bytes(), nil
}

func (a *mockSnapshotter) UnmarshalSnapshot(p []byte) error {
	if err := gob.NewDecoder(bytes.NewReader(p)).Decode(&a.mockState); err != nil {
		return fmt.Errorf("gob: %w", err)
	}
	return nil
}
