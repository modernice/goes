package test

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/encoding"
)

// FooEventData is a testing event.Data.
type FooEventData struct{ A string }

// BarEventData is a testing event.Data.
type BarEventData struct{ A string }

// BazEventData is a testing event.Data.
type BazEventData struct{ A string }

// UnregisteredEventData is a testing event.Data that's not registered in the
// Encoder returned by NewEncoder.
type UnregisteredEventData struct{ A string }

// NewEncoder returns a "gob" event.Encoder with registered "foo", "bar" and
// "baz" events.
func NewEncoder() event.Encoder {
	enc := encoding.NewGobEncoder()
	enc.Register("foo", func() event.Data {
		return FooEventData{}
	})
	enc.Register("bar", func() event.Data {
		return BarEventData{}
	})
	enc.Register("baz", func() event.Data {
		return BazEventData{}
	})
	return enc
}
