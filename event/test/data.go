package test

import (
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

// FooEventData is a testing event data.
type FooEventData struct{ A string }

// BarEventData is a testing event data.
type BarEventData struct{ A string }

// BazEventData is a testing event data.
type BazEventData struct{ A string }

// FoobarEventData is a testing event data.
type FoobarEventData struct{ A int }

// UnregisteredEventData is a testing event data that's not registered in the
// Encoder returned by NewEncoder.
type UnregisteredEventData struct{ A string }

// NewEncoder returns a "gob" event.Encoding with registered "foo", "bar" and
// "baz" events.
func NewEncoder() *codec.GobRegistry[any] {
	reg := codec.Gob(event.NewRegistry())
	reg.GobRegister("foo", func() any {
		return FooEventData{}
	})
	reg.GobRegister("bar", func() any {
		return BarEventData{}
	})
	reg.GobRegister("baz", func() any {
		return BazEventData{}
	})
	reg.GobRegister("foobar", func() any {
		return FoobarEventData{}
	})
	return reg
}
