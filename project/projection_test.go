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
