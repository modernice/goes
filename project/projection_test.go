package project_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestProjection_LatestEvent(t *testing.T) {
	p := project.NewProjection()
	evt := event.New("foo", test.FooEventData{})
	p.PostApplyEvent(evt)

	if !event.Equal(evt, p.LatestEvent()) {
		t.Fatalf("LatestEvent should return %v; got %v", evt, p.LatestEvent())
	}
}
