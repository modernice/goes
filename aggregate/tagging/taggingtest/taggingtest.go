// Package taggingtest provides testing helpers for the tagging module.
package taggingtest

//go:generate mockgen -source=taggingtest.go -destination=./mock_taggingtest/taggingtest.go

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tagging"
)

// TestingT is used to mock *testing.T.
type TestingT interface {
	Fatalf(string, ...interface{})
}

// Tagger provides a `HasTag` method. Ususally an Aggregate that embeds *tagging.Tagger.
type Tagger interface {
	HasTag(string) bool
}

// Aggregate tests that an Aggregate correctly implements tagging.
//
// The test fails in the following cases:
// 	- Aggregate does not implement Tagger (HasTag method)
// 	- Aggregate does not apply tagging events
//
// When embedding a *tagging.Tagger into an Aggregate, make sure to call
// ApplyEvent on the embedded *tagging.Tagger like this:
//
//	type Foo struct {
//		*aggregate.Base
//		*tagging.Tagger
//	}
//
//	func (f *Foo) ApplyEvent(evt event.Event) {
//		f.Tagger.ApplyEvent(evt)
//	}
func Aggregate(t TestingT, a aggregate.Aggregate) {
	tagger, ok := a.(Tagger)
	if !ok {
		t.Fatalf("[tagging] %q Aggregate does not implement taggingtest.Tagger. Forgot to embed *tagging.Tagger?", a.AggregateName())
		return
	}

	missingApply := func() {
		t.Fatalf("[tagging] %q Aggregate does not apply tagging events. Forgot to implement `ApplyEvent`?", a.AggregateName())
	}

	tagging.Tag(a, "foo", "bar", "baz")
	if !(tagger.HasTag("foo") && tagger.HasTag("bar") && tagger.HasTag("baz")) {
		missingApply()
		return
	}

	tagging.Untag(a, "foo")
	if tagger.HasTag("foo") {
		missingApply()
		return
	}
}
