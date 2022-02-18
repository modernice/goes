package snapshot

import (
	"encoding"
	"errors"

	"github.com/modernice/goes"
)

// ErrUnimplemented is returned when trying to marshal or unmarshal a snapshot
// into an aggregate that does not implement one of the supported marshalers.
var ErrUnimplemented = errors.New("aggregate does not implement (Un)Marshaler, encoding.Binary(Un)Marshaler or encoding.Text(Un)Marshaler")

// A Marshaler can encode itself into bytes. Aggregates must implement Marshaler
// & Unmarshaler for Snapshots to work.
//
// Example using encoding/gob:
//
//	type foo struct {
//		aggregate.Aggregate
//		state
//	}
//
//	type state struct {
//		Name string
//		Age uint8
//	}
//
//	func (f *foo) MarshalSnapshot() ([]byte, error) {
//		var buf bytes.Buffer
//		err := gob.NewEncoder(&buf).Encode(f.state)
//		return buf.Bytes(), err
//	}
//
//	func (f *foo) UnmarshalSnapshot(p []byte) error {
//		return gob.NewDecoder(bytes.NewReader(p)).Decode(&f.state)
//	}
type Marshaler interface {
	MarshalSnapshot() ([]byte, error)
}

// An Unmarshaler can decode itself from bytes.
type Unmarshaler interface {
	UnmarshalSnapshot([]byte) error
}

// Marshal encodes the given aggregate into a byte slice. If a implements
// Marshaler, a.MarshalSnapshot() is returned. If a implements
// encoding.BinaryMarshaler, a.MarshalBinary() is returned and if a implements
// encoding.TextMarshaler, a.MarshalText is returned. If a implements none of
// these interfaces, Marshal uses encoding/gob to marshal the snapshot.
func Marshal(a any) ([]byte, error) {
	if m, ok := a.(Marshaler); ok {
		return m.MarshalSnapshot()
	}

	if m, ok := a.(encoding.BinaryMarshaler); ok {
		return m.MarshalBinary()
	}

	if m, ok := a.(encoding.TextMarshaler); ok {
		return m.MarshalText()
	}

	return nil, ErrUnimplemented
}

// Target is a snapshot target. This should be an aggregate that implements the
// SetVersion function.
type Target interface {
	SetVersion(int)
}

// Unmarshal decodes the Snapshot s into the Aggregate a by calling
// a.UnmarshalSnapshot(s.State()). Unmarshal returns nil if the Aggregate does
// not implement Unmarshaler.
//
// TODO: return an error if the Aggregate does not implement Unmarshaler?

// Unmarshal decodes the given snapshot into the given aggregate. If a
// implements Unmarshaler, a.UnmarshalSnapshot() is returned. If a implements
// encoding.BinaryMarshaler, a.UnmarshalBinary() is returned and if a implements
// encoding.TextUnmarshaler, a.UnmarshalText() is returned. If a implements none
// of these interfaces, encoding/gob is used to unmarshal the snapshot.
func Unmarshal[ID goes.ID](s Snapshot[ID], a Target) error {
	a.SetVersion(s.AggregateVersion())

	if u, ok := a.(Unmarshaler); ok {
		return u.UnmarshalSnapshot(s.State())
	}

	if u, ok := a.(encoding.BinaryUnmarshaler); ok {
		return u.UnmarshalBinary(s.State())
	}

	if u, ok := a.(encoding.TextUnmarshaler); ok {
		return u.UnmarshalText(s.State())
	}

	return ErrUnimplemented
}
