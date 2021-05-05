package test

import (
	"github.com/modernice/goes/event"
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
func NewEncoder() event.Registry {
	reg := event.NewRegistry()
	reg.Register("foo", func() event.Data {
		return FooEventData{}
	})
	reg.Register("bar", func() event.Data {
		return BarEventData{}
	})
	reg.Register("baz", func() event.Data {
		return BazEventData{}
	})
	return reg
}
