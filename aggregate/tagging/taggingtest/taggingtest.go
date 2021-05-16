// Package taggingtest provides testing helpers for the tagging module.
package taggingtest

//go:generate mockgen -source=taggingtest.go -destination=./mock_taggingtest/taggingtest.go

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

// TestingT is used to mock *testing.T.
type TestingT interface {
	Fatalf(string, ...interface{})
}

// Tagger provides a `HasTag` method. Usually an Aggregate that embeds *tagging.Tagger.
// A Tagger has tags.
//
// Tagger is usually implemented by embedding *tagging.Tagger into an aggregate.
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

// StoreFactory creates tagging stores.
type StoreFactory func(event.Encoder) tagging.Store

// Store tests a tagging store implementation.
func Store(t *testing.T, newStore StoreFactory) {
	t.Run("Update", func(t *testing.T) {
		testStore_Update(t, newStore)
	})
	t.Run("TaggedWith", func(t *testing.T) {
		testStore_TaggedWith(t, newStore)
	})
}

func testStore_Update(t *testing.T, newStore StoreFactory) {
	enc := test.NewEncoder()
	store := newStore(enc)

	name := "foo"
	id := uuid.New()
	tags := []string{"foo", "bar", "baz"}
	if err := store.Update(context.Background(), name, id, tags); err != nil {
		t.Fatalf("Update failed with %q", err)
	}

	result, err := store.Tags(context.Background(), name, id)
	if err != nil {
		t.Fatalf("Tags failed with %q", err)
	}

	if len(result) != len(tags) {
		t.Fatalf("Tags should return %d tags; got %d", len(tags), len(result))
	}

	for _, tag := range tags {
		var found bool
		for _, tag2 := range result {
			if tag2 == tag {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Tags should return tag %q", tag)
		}
	}
}

func testStore_TaggedWith(t *testing.T, newStore StoreFactory) {
	enc := test.NewEncoder()
	store := newStore(enc)

	aggregates := []tagging.Aggregate{
		{
			Name: "foo",
			ID:   uuid.New(),
		},
		{
			Name: "bar",
			ID:   uuid.New(),
		},
		{
			Name: "baz",
			ID:   uuid.New(),
		},
	}

	tags := []string{"foo", "bar", "baz"}
	for _, a := range aggregates {
		if err := store.Update(context.Background(), a.Name, a.ID, tags); err != nil {
			t.Fatalf("Update failed with %q", err)
		}
	}

	for _, tag := range tags {
		tagged, err := store.TaggedWith(context.Background(), tag)
		if err != nil {
			t.Fatalf("TaggedWith failed with %q", err)
		}

		if len(tagged) != len(aggregates) {
			t.Fatalf("TaggedWith should return %d aggregates; got %d", len(aggregates), len(tagged))
		}

		for _, a := range aggregates {
			var found bool
			for _, tagged := range tagged {
				if tagged == a {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("TaggedWith should return %v", a)
			}
		}
	}

	tagged, err := store.TaggedWith(context.Background(), tags...)
	if err != nil {
		t.Fatalf("TaggedWith failed with %q", err)
	}

	if len(tagged) != len(aggregates) {
		t.Fatalf("TaggedWith should return %d aggregates; got %d", len(aggregates), len(tagged))
	}

	for _, a := range aggregates {
		var found bool
		for _, tagged := range tagged {
			if a == tagged {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("TaggedWith should return %v", a)
		}
	}

	tagged, err = store.TaggedWith(context.Background(), "foobar")
	if err != nil {
		t.Fatalf("TaggedWith failed with %q", err)
	}

	if len(tagged) != 0 {
		t.Fatalf("TaggedWith should no aggregates; got %d", len(tagged))
	}

	tagged, err = store.TaggedWith(context.Background())
	if err != nil {
		t.Fatalf("TaggedWith failed with %q", err)
	}

	if len(tagged) != 3 {
		t.Fatalf("TaggedWith should %d aggregates; got %d", len(aggregates), len(tagged))
	}
}
