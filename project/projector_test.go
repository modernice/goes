package project_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/project"
)

func TestProjector_Project(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.FooEventData{}),
		event.New("baz", test.FooEventData{}),
	}

	ex := newMockProjection()

	err := proj.Project(context.Background(), events, ex)
	if err != nil {
		t.Fatalf("failed to apply Projection: %v", err)
	}

	if !ex.hasApplied(events...) {
		t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
	}

	wantLatest := events[len(events)-1]
	if !wantLatest.Time().Equal(ex.LatestEventTime()) {
		t.Fatalf("LatestEventTime should return %v; got %v", wantLatest.Time(), ex.LatestEventTime())
	}
}

func TestProjector_Continuously(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := newMockProjection()

	appliedJobs := make(chan project.Job)

	errs, err := proj.Continuously(
		ctx,
		[]string{"foo", "bar"},
		func(job project.Job) error {
			if err := job.Apply(job.Context(), ex); err != nil {
				return fmt.Errorf("failed to apply Projection Job: %v", err)
			}
			appliedJobs <- job
			return nil
		},
	)
	if err != nil {
		t.Fatalf("failed to subscribe to Projections: %v", err)
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate("foobar", uuid.New(), 1)),
		event.New("bar", test.BarEventData{}, event.Aggregate("foobar", uuid.New(), 1)),
		event.New("baz", test.BazEventData{}, event.Aggregate("barbaz", uuid.New(), 1)),
	}

	publishError := make(chan error)
	go func() {
		if err := bus.Publish(ctx, events...); err != nil {
			publishError <- err
		}
	}()

	var jobs []project.Job
	for i := 0; i < 2; i++ {
		select {
		case err := <-publishError:
			t.Fatalf("failed to publish Event: %v", err)
		case err, ok := <-errs:
			if !ok {
				t.Fatal("error channel shouldn't be closed!")
			}
			t.Fatalf("Projection failed: %v", err)
		case j := <-appliedJobs:
			jobs = append(jobs, j)
		}
	}

	<-time.After(50 * time.Millisecond)

	cancel()

	select {
	case err := <-publishError:
		t.Fatalf("failed to publish Event: %v", err)
	case _, ok := <-errs:
		if ok {
			t.Fatal("error channel should be closed!")
		}
	}

	if !ex.hasApplied(events[:2]...) {
		t.Fatalf("applied Events should be %v; got %v", events[:2], ex.applied)
	}

	for i, j := range jobs {
		jobEvents, err := j.Events(context.Background(), ex)
		if err != nil {
			t.Fatal(err)
		}

		test.AssertEqualEvents(t, []event.Event{events[i]}, jobEvents)

		aggregates, err := j.Aggregates(context.Background(), ex)
		if err != nil {
			t.Fatal(err)
		}

		wantAggregates := map[string][]uuid.UUID{
			"foobar": {events[i].AggregateID()},
		}
		if !reflect.DeepEqual(wantAggregates, aggregates) {
			t.Fatalf("Aggregates should return %v; got %v", wantAggregates, aggregates)
		}

		ids, err := j.AggregatesOf(context.Background(), "foobar", ex)
		if err != nil {
			t.Fatal(err)
		}

		wantIDs := []uuid.UUID{events[i].AggregateID()}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "foobar", wantIDs, ids)
		}

		ids, err = j.AggregatesOf(context.Background(), "invalid", ex)
		if err != nil {
			t.Fatal(err)
		}

		if ids != nil {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "invalid", nil, ids)
		}

		id, err := j.Aggregate(context.Background(), "foobar", ex)
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "foobar", events[0].AggregateID(), id)
		}

		id, err = j.Aggregate(context.Background(), "invalid", ex)
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "invalud", uuid.Nil, id)
		}
	}
}

func TestProjector_Periodically(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate("foobar", uuid.New(), 1)),
		event.New("bar", test.BarEventData{}, event.Aggregate("foobar", uuid.New(), 1)),
		event.New("baz", test.BazEventData{}, event.Aggregate("barbaz", uuid.New(), 1)),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	var projections []*mockProjection
	var jobs []project.Job

	errs, err := proj.Periodically(
		ctx,
		80*time.Millisecond, // use 80ms so that Projections are run at least 2 times in 200ms
		[]string{"foo", "bar"},
		func(job project.Job) error {
			ex := newMockProjection()
			if err := job.Apply(job.Context(), ex); err != nil {
				return fmt.Errorf("failed to apply Projection Job: %w", err)
			}
			projections = append(projections, ex)
			jobs = append(jobs, job)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("failed to subscribe to Projections: %v", err)
	}

	go func() {
		<-time.After(200 * time.Millisecond)
		cancel()
	}()

	for err := range errs {
		t.Fatalf("Projection failed: %v", err)
	}

	if len(projections) < 2 {
		t.Fatalf("at least %d Projections should have been run; got %d", 2, len(projections))
	}

	for _, p := range projections {
		if !p.hasApplied(events[:2]...) {
			t.Fatalf("applied Events should be %v; got %v", events, p.applied)
		}

		if !p.LatestEventTime().Equal(events[1].Time()) {
			t.Fatalf("LatestEventTime should return %v; got %v", events[1].Time(), p.LatestEventTime())
		}
	}

	for i, j := range jobs {
		jobEvents, err := j.Events(context.Background(), projections[i])
		if err != nil {
			t.Fatal(err)
		}

		test.AssertEqualEvents(t, events[:2], jobEvents)

		aggregates, err := j.Aggregates(context.Background(), projections[i])
		if err != nil {
			t.Fatal(err)
		}

		wantAggregates := map[string][]uuid.UUID{
			"foobar": {events[0].AggregateID(), events[1].AggregateID()},
		}
		if !reflect.DeepEqual(wantAggregates, aggregates) {
			t.Fatalf("Aggregates should return %v; got %v", wantAggregates, aggregates)
		}

		ids, err := j.AggregatesOf(context.Background(), "foobar", projections[i])
		if err != nil {
			t.Fatal(err)
		}

		wantIDs := []uuid.UUID{events[0].AggregateID(), events[1].AggregateID()}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "foobar", wantIDs, ids)
		}

		ids, err = j.AggregatesOf(context.Background(), "invalid", projections[i])
		if err != nil {
			t.Fatal(err)
		}

		if ids != nil {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "invalid", nil, ids)
		}

		id, err := j.Aggregate(context.Background(), "foobar", projections[i])
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "foobar", events[0].AggregateID(), id)
		}

		id, err = j.Aggregate(context.Background(), "invalid", projections[i])
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "invalud", uuid.Nil, id)
		}
	}
}

func TestProjector_Periodically_withLatestEvent(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	ex := newMockProjection()
	ex.applied = events[:1]
	ex.PostApplyEvent(events[0])

	errs, err := proj.Periodically(
		ctx,
		50*time.Millisecond,
		[]string{"foo", "bar", "baz"},
		func(job project.Job) error {
			defer cancel()
			if err := job.Apply(job.Context(), ex); err != nil {
				return fmt.Errorf("failed to apply Projection Job: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("failed to subscribe to Projections: %v", err)
	}

	for err := range errs {
		t.Fatalf("Projection failed: %v", err)
	}

	if !ex.hasApplied(events...) {
		t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
	}

	if ex.hasDuplicates() {
		t.Fatalf("Projection has duplicate applied Events. Does the Projector add query filters to exlcude already applied Events?")
	}
}

func TestFilter(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	publishError := make(chan error)
	go func() {
		if err := bus.Publish(context.Background(), events...); err != nil {
			publishError <- err
		}
	}()

	ex := newMockProjection()

	appliedJobs := make(chan project.Job, 2)

	errs, err := proj.Continuously(
		ctx,
		[]string{"foo", "bar", "baz"},
		func(j project.Job) error {
			if err := j.Apply(j.Context(), ex); err != nil {
				return fmt.Errorf("failed to apply Projection: %w", err)
			}
			appliedJobs <- j
			return nil
		},
		project.Filter(query.New(query.Name("foo", "bar"))),
	)
	if err != nil {
		t.Fatalf("failed to subscribe to Projections: %v", err)
	}

	for range events[:2] {
		select {
		case err := <-publishError:
			t.Fatalf("failed to publish Event: %v", err)
		case err, ok := <-errs:
			if !ok {
				t.Fatal("error channel shouldn't be closed!")
			}
			t.Fatalf("Projection failed: %v", err)
		case <-appliedJobs:
		}
	}

	if !ex.hasApplied(events[:2]...) {
		t.Fatalf("applied Events should be %v; got %v", events[:2], ex.applied)
	}

	if ex.hasApplied(events[2]) {
		t.Fatalf("%v should not have been applied!", events[2])
	}
}

func TestFromBase(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	proj := project.NewProjector(bus, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	ex := newMockProjection()

	appliedJobs := make(chan project.Job)

	errs, err := proj.Continuously(
		ctx,
		[]string{"foo", "bar", "baz"},
		func(job project.Job) error {
			if err := job.Apply(job.Context(), ex); err != nil {
				return fmt.Errorf("failed to apply Projection Job: %v", err)
			}
			appliedJobs <- job
			return nil
		},
		project.FromBase(),
	)
	if err != nil {
		t.Fatalf("failed to subscribe to Projections: %v", err)
	}

	publishError := make(chan error)
	go func() {
		if err := bus.Publish(ctx, events[0]); err != nil {
			publishError <- err
		}
	}()

	select {
	case err := <-publishError:
		t.Fatalf("failed to publish Event: %v", err)
	case err, ok := <-errs:
		if !ok {
			t.Fatal("error channel shouldn't be closed!")
		}
		t.Fatalf("Projection failed: %v", err)
	case <-appliedJobs:
	}

	if !ex.hasApplied(events...) {
		t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
	}
}

type mockProjection struct {
	*project.Projection

	applied []event.Event
}

func newMockProjection() *mockProjection {
	return &mockProjection{
		Projection: project.NewProjection(),
	}
}

func (p *mockProjection) ApplyEvent(evt event.Event) {
	p.applied = append(p.applied, evt)
}

func (p *mockProjection) hasApplied(events ...event.Event) bool {
	for _, evt := range events {
		var applied bool
		for _, evt2 := range p.applied {
			if evt.ID() == evt2.ID() {
				applied = true
				break
			}
		}
		if !applied {
			return false
		}
	}
	return true
}

func (p *mockProjection) hasDuplicates() bool {
	for i, evt := range p.applied {
		for _, applied := range p.applied[i+1:] {
			if event.Equal(evt, applied) {
				return true
			}
		}
	}
	return false
}
