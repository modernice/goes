package test

import (
	"github.com/modernice/goes/codec"
)

// FooEventData is a testing event.Data.
type FooEventData struct{ A string }

// BarEventData is a testing event.Data.
type BarEventData struct{ A string }

// BazEventData is a testing event.Data.
type BazEventData struct{ A string }

// FoobarEventData is a testing event.Data.
type FoobarEventData struct{ A int }

// UnregisteredEventData is a testing event.Data that's not registered in the
// Encoder returned by NewEncoder.
type UnregisteredEventData struct{ A string }

// NewEncoder returns a "gob" event.Encoder with registered "foo", "bar" and
// "baz" events.
func NewEncoder() *codec.Registry {
	reg := codec.Gob(codec.New())
	reg.GobRegister("foo", func() interface{} {
		return FooEventData{}
	})
	reg.GobRegister("bar", func() interface{} {
		return BarEventData{}
	})
	reg.GobRegister("baz", func() interface{} {
		return BazEventData{}
	})
	reg.GobRegister("foobar", func() interface{} {
		return FoobarEventData{}
	})
	return reg.Registry
}
