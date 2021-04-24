package project_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestProjection_LatestEventTime(t *testing.T) {
	p := project.NewProjection()
	evt := event.New("foo", test.FooEventData{})
	p.PostApplyEvent(evt)

	if !evt.Time().Equal(p.LatestEventTime()) {
		t.Fatalf("LatestEventTime should return %v; got %v", evt.Time(), p.LatestEventTime())
	}
}
