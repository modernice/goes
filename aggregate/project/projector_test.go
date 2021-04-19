package project_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/project"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
)

type mockProjection struct {
	project.Projection

	events []event.Event
}

func TestProjector_Project(t *testing.T) {
	proj, repo, _ := newProjector()

	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 5, xevent.ForAggregate(a))
	a.TrackChange(events...)

	if err := repo.Save(context.Background(), a); err != nil {
		t.Fatalf("Save shouldn't fail; failed with %q", err)
	}

	mp := newMockProjection(a.AggregateID())
	if err := proj.Project(context.Background(), mp); err != nil {
		t.Errorf("Project shouldn't fail; failed with %q", err)
	}

	test.AssertEqualEvents(t, mp.events, events)

	if mp.AggregateVersion() != 5 {
		t.Errorf("Projection should be at version %d; is at version %d", 5, mp.AggregateVersion())
	}
}

func (mp *mockProjection) ApplyEvent(evt event.Event) {
	mp.events = append(mp.events, evt)
}

func newMockProjection(id uuid.UUID) *mockProjection {
	return &mockProjection{
		Projection: project.New("foo", id),
	}
}

func newProjector() (project.Projector, aggregate.Repository, event.Store) {
	events := memstore.New()
	repo := repository.New(events)
	proj := project.NewProjector(events)
	return proj, repo, events
}
