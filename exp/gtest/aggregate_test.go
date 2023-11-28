package gtest_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/exp/gtest"
)

func TestTransition(t *testing.T) {
	type data struct {
		Foo string
		Bar int
	}

	foo := aggregate.New("foo", uuid.New())

	d := data{Foo: "foo", Bar: 42}
	aggregate.Next(foo, "foo", d)

	gtest.Transition("foo", d).Run(t, foo)
}

func TestTransition_sameEventNameOtherData(t *testing.T) {
	type data struct {
		Foo string
		Bar bool
	}

	foo := aggregate.New("foo", uuid.New())

	d1 := data{Foo: "foo", Bar: false}
	d2 := data{Foo: "foo", Bar: true}

	aggregate.Next(foo, "foo", d1)
	aggregate.Next(foo, "foo", d2)

	gtest.Transition("foo", d1).Run(t, foo)
	gtest.Transition("foo", d2).Run(t, foo)
}

type comparableData struct {
	Foo string
	Bar int
	Baz []bool
}

// Equal checks if two instances of comparableData are equivalent by comparing
// their Foo, Bar, and Baz fields. It returns true if all corresponding fields
// between the two instances are equal, false otherwise.
func (d comparableData) Equal(d2 comparableData) bool {
	return d.Foo == d2.Foo && d.Bar == d2.Bar && slices.Equal(d.Baz, d2.Baz)
}

func TestTransitionWithEqual(t *testing.T) {
	foo := aggregate.New("foo", uuid.New())

	d := comparableData{Foo: "foo", Bar: 42, Baz: []bool{true, false}}
	aggregate.Next(foo, "foo", d)

	gtest.TransitionWithEqual("foo", d).Run(t, foo)
}
