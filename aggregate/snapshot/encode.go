package snapshot

import (
	"github.com/modernice/goes/aggregate"
)

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

// Marshal encodes the given Aggregate into a byte slice. Implementations of
// Aggregate must implement Marshaler for Marshal to return anything other than
// nil. If the Aggregate does not implement Marshaler, Marshal returns nil, nil.
func Marshal(a aggregate.Aggregate) ([]byte, error) {
	if m, ok := a.(Marshaler); ok {
		return m.MarshalSnapshot()
	}
	return nil, nil
}

// Unmarshal decodes the Snapshot in p into the Aggregate a by calling
// a.UnmarshalSnapshot(p). Unmarshal always returns nil if the Aggregate does
// not implement Unmarshaler.
func Unmarshal(s Snapshot, a aggregate.Aggregate) error {
	a.SetVersion(s.AggregateVersion())
	if u, ok := a.(Unmarshaler); ok {
		if err := u.UnmarshalSnapshot(s.State()); err != nil {
			return err
		}
	}
	return nil
}