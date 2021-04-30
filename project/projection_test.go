package project_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestProjection_LatestEventTime(t *testing.T) {
	p := project.NewProjection()
	now := time.Now()
	evt := event.New("foo", test.FooEventData{}, event.Time(now))
	p.PostApplyEvent(evt)

	if !evt.Time().Equal(p.LatestEventTime()) {
		t.Fatalf("LatestEventTime should return %v; got %v", now, p.LatestEventTime())
	}
}

func TestApply_guarded(t *testing.T) {
	g := newGuarded("foo", "bar")

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	if err := project.Apply(events, g); err != nil {
		t.Fatalf("failed to apply projection: %v", err)
	}

	test.AssertEqualEvents(t, events[:2], g.applied)
}

type guarded struct {
	*project.Projection

	allowed []string
	applied []event.Event
}

func newGuarded(allowed ...string) *guarded {
	return &guarded{
		Projection: project.NewProjection(),
		allowed:    allowed,
	}
}

func (g *guarded) Guard(evt event.Event) bool {
	for _, name := range g.allowed {
		if evt.Name() == name {
			return true
		}
	}
	return false
}

func (g *guarded) ApplyEvent(evt event.Event) {
	g.applied = append(g.applied, evt)
}
