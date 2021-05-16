package tagging

import (
	"sort"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// Tagger can be embedded into structs to make them taggable.
type Tagger []string

// HasTag returns whether t has the given tag.
func (t Tagger) HasTag(tag string) bool {
	for _, t2 := range t {
		if tag == t2 {
			return true
		}
	}
	return false
}

// Tags returns t as a string slice.
func (t Tagger) Tags() []string {
	return t
}

// ApplyEvent applies tagging events. Aggregates that embed *Tagger should call
// t.ApplyEvent from within their own ApplyEvent method:
//
//	type Foo struct {
//		*aggregate.Base
//		*tagging.Tagger
//	}
//
//	func (f *Foo) ApplyEvent(evt event.Event) {
//		f.Tagger.ApplyEvent(evt)
//	}
func (t *Tagger) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Tagged:
		t.tag(evt)
	case Untagged:
		t.untag(evt)
	}
}

func (t *Tagger) tag(evt event.Event) {
	defer t.sort()
	data := evt.Data().(TaggedEvent)
	for _, tag := range data.Tags {
		if !t.HasTag(tag) {
			*t = append(*t, tag)
		}
	}
}

func (t *Tagger) untag(evt event.Event) {
	defer t.sort()
	data := evt.Data().(UntaggedEvent)
	for _, tag := range data.Tags {
		for i, existing := range *t {
			if existing != tag {
				continue
			}
			*t = append((*t)[:i], (*t)[i+1:]...)
			break
		}
	}
}

func (t *Tagger) sort() {
	sort.Slice(*t, func(i, j int) bool { return (*t)[i] <= (*t)[j] })
}

// Tag adds tags to a and returns the applied "goes.tagging.tagged" Event.
func Tag(a aggregate.Aggregate, tags ...string) event.Event {
	return aggregate.NextEvent(a, Tagged, TaggedEvent{Tags: unique(tags...)})
}

// Untag removes tags from a and returns the applied "goes.tagging.untagged" Event.
func Untag(a aggregate.Aggregate, tags ...string) event.Event {
	return aggregate.NextEvent(a, Untagged, UntaggedEvent{Tags: unique(tags...)})
}

func unique(tags ...string) []string {
	lookup := make(map[string]bool)
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if lookup[tag] {
			continue
		}
		lookup[tag] = true
		out = append(out, tag)
	}
	return out
}
