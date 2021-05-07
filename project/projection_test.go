package project_test

import (
	"testing"
	"time"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestProgressor_ProgressProjection(t *testing.T) {
	var p project.Progressor

	now := time.Now()

	p.ProgressProjection(now)

	if now.UnixNano() != p.LatestEventTime {
		t.Fatalf("LatestEventTime should be %v; got %v", now.UnixNano(), p.LatestEventTime)
	}

	latest := p.ProjectionProgress()

	if !now.Equal(latest) {
		t.Fatalf("LatestEvent should return Time %v; got %v", now, latest)
	}
}

func TestApply_progressor(t *testing.T) {
	p := &progressed{Progressor: &project.Progressor{}}
	now := time.Now()

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Time(now)),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Minute))),
		event.New("foo", test.FooEventData{}, event.Time(now.Add(time.Hour))),
	}

	if err := project.Apply(events, p); err != nil {
		t.Fatalf("Apply shouldn't fail; failed with %q", err)
	}

	if p.LatestEventTime != now.Add(time.Hour).UnixNano() {
		t.Fatalf("LatestEventTime should be %v; got %v", now.Add(time.Hour).UnixNano(), p.LatestEventTime)
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
	allowed []string
	applied []event.Event
}

func newGuarded(allowed ...string) *guarded {
	return &guarded{
		allowed: allowed,
	}
}

func (g *guarded) GuardProjection(evt event.Event) bool {
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

type progressed struct {
	*project.Progressor
}

func (p *progressed) ApplyEvent(event.Event) {}
