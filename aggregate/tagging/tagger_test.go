package tagging_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/event"
)

type tagger struct {
	*aggregate.Base
	*tagging.Tagger
}

func newTagAggregate() *tagger {
	return &tagger{
		Base:   aggregate.New("foo", uuid.New()),
		Tagger: &tagging.Tagger{},
	}
}

func (a *tagger) ApplyEvent(evt event.Event) {
	a.Tagger.ApplyEvent(evt)
}

func TestTags(t *testing.T) {
	foo := newTagAggregate()

	if foo.HasTag("foo") {
		t.Fatalf("should not have %q tag", "foo")
	}

	if foo.HasTag("bar") {
		t.Fatalf("should not have %q tag", "bar")
	}

	tagging.Tag(foo, "foo", "bar")

	if !foo.HasTag("foo") {
		t.Fatalf("should have %q tag", "foo")
	}

	if !foo.HasTag("bar") {
		t.Fatalf("should have %q tag", "bar")
	}

	tagging.Untag(foo, "bar")

	if !foo.HasTag("foo") {
		t.Fatalf("should have %q tag", "foo")
	}

	if foo.HasTag("bar") {
		t.Fatalf("should not have %q tag", "bar")
	}

	tagging.Tag(foo, "foo", "baz")
	tagging.Untag(foo, "foo", "bar")

	if foo.HasTag("foo") {
		t.Fatalf("should not have %q tag", "foo")
	}

	if foo.HasTag("bar") {
		t.Fatalf("should not have %q tag", "bar")
	}

	if !foo.HasTag("baz") {
		t.Fatalf("should have %q tag", "baz")
	}

	tagging.Tag(foo, "foo", "bar", "baz")

	wantTags := []string{"bar", "baz", "foo"}
	if !reflect.DeepEqual(foo.Tagger.Tags(), wantTags) {
		t.Fatalf("aggregate has wrong tags. want=%v got=%v", wantTags, foo.Tagger.Tags())
	}
}
