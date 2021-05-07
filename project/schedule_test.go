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

func TestContinuously(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()

	s := project.Continuously(bus, store, []string{"foo", "bar"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := newMockProjection()

	appliedJobs := make(chan project.Job)

	errs, err := s.Subscribe(
		ctx,
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
		for _, evt := range events {
			if err := bus.Publish(ctx, evt); err != nil {
				publishError <- err
			}
			<-time.After(10 * time.Millisecond)
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
		jobEvents, err := j.Events(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		test.AssertEqualEvents(t, []event.Event{events[i]}, jobEvents)

		jobEvents, err = j.EventsFor(context.Background(), ex)
		if err != nil {
			t.Fatal(err)
		}

		test.AssertEqualEvents(t, []event.Event{events[i]}, jobEvents)

		aggregates, err := j.Aggregates(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		wantAggregates := map[string][]uuid.UUID{
			"foobar": {events[i].AggregateID()},
		}
		if !reflect.DeepEqual(wantAggregates, aggregates) {
			t.Fatalf("Aggregates should return %v; got %v", wantAggregates, aggregates)
		}

		ids, err := j.AggregatesOf(context.Background(), "foobar")
		if err != nil {
			t.Fatal(err)
		}

		wantIDs := []uuid.UUID{events[i].AggregateID()}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "foobar", wantIDs, ids)
		}

		ids, err = j.AggregatesOf(context.Background(), "invalid")
		if err != nil {
			t.Fatal(err)
		}

		if ids != nil {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "invalid", nil, ids)
		}

		id, err := j.Aggregate(context.Background(), "foobar")
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "foobar", events[0].AggregateID(), id)
		}

		id, err = j.Aggregate(context.Background(), "invalid")
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "invalud", uuid.Nil, id)
		}
	}
}

func TestContinuously_Trigger(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	eventNames := []string{"foo", "bar", "baz"}
	s := project.Continuously(bus, store, eventNames)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan project.Job, 3)
	errs := make(chan error)

	for i := 0; i < 3; i++ {
		sub, err := s.Subscribe(ctx, func(j project.Job) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- j:
				return nil
			}
		})
		if err != nil {
			t.Fatalf("failed to subscribe to Schedule: %v", err)
		}
		go func() {
			for err := range sub {
				select {
				case <-ctx.Done():
					return
				case errs <- err:
				}
			}
		}()
	}

	select {
	case j := <-jobs:
		t.Fatalf("unexpected Job: %v", j)
	case err, ok := <-errs:
		if !ok {
			errs = nil
			break
		}
		t.Fatal(err)
	default:
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	triggerError := make(chan error)
	go func() {
		if err := s.Trigger(ctx); err != nil {
			triggerError <- fmt.Errorf("failed to trigger Schedule: %w", err)
		}
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out")
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			t.Fatal(err)
		case err := <-triggerError:
			t.Fatal(err)
		case j := <-jobs:
			evts, err := j.Events(j.Context())
			if err != nil {
				t.Fatalf("failed to fetch Job Events: %v", err)
			}
			test.AssertEqualEventsUnsorted(t, evts, events)
		}
	}
}

func TestContinuously_progressor(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()
	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"})

	now := time.Now()
	id := uuid.New()

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate("foo", id, 0), event.Time(now.Add(-time.Minute))),
		event.New("bar", test.BarEventData{}, event.Aggregate("foo", id, 0), event.Time(now)),
		event.New("baz", test.BarEventData{}, event.Aggregate("foo", id, 0), event.Time(now.Add(time.Minute))),
		event.New("foo", test.FooEventData{}, event.Aggregate("foo", id, 0), event.Time(now.Add(time.Hour))),
	}

	for _, evt := range events {
		if err := store.Insert(context.Background(), evt); err != nil {
			t.Fatalf("failed to insert Event: %v", err)
		}
	}

	ex := newMockProjection()
	ex.LatestEventTime = events[1].Time().UnixNano()
	ex.applied = events[:2]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	applied := make(chan project.Job)

	errs, err := s.Subscribe(ctx, func(j project.Job) error {
		if err := j.Apply(j.Context(), ex); err != nil {
			return fmt.Errorf("failed to apply Job: %w", err)
		}
		applied <- j
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to Schedule: %v", err)
	}

	go s.Trigger(ctx)

	select {
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	case err, ok := <-errs:
		if !ok {
			t.Fatalf("error channel shouldn't be closed")
			return
		}
		t.Fatal(err)
	case <-applied:
		if !ex.hasApplied(events...) {
			t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
		}
		if ex.hasDuplicates() {
			t.Fatalf("projection has duplicate applied Events")
		}
	}
}

func TestPeriodically(t *testing.T) {
	store := memstore.New()
	s := project.Periodically(store, 80*time.Millisecond, []string{"foo", "bar"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	events := []event.Event{
		event.New("foo", test.FooEventData{}, event.Aggregate("foobar", uuid.New(), 1), event.Time(now)),
		event.New("bar", test.BarEventData{}, event.Aggregate("foobar", uuid.New(), 1), event.Time(now.Add(time.Minute))),
		event.New("baz", test.BazEventData{}, event.Aggregate("barbaz", uuid.New(), 1), event.Time(now.Add(time.Hour))),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	var projections []*mockProjection
	var jobs []project.Job

	errs, err := s.Subscribe(
		ctx,
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
		<-time.After(300 * time.Millisecond)
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

		// if !p.LatestEventTime().Equal(events[1].Time()) {
		// 	t.Fatalf("LatestEventTime should return %v; got %v", events[1].Time(), p.LatestEventTime())
		// }
	}

	for _, j := range jobs {
		jobEvents, err := j.Events(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		test.AssertEqualEvents(t, events[:2], jobEvents)

		aggregates, err := j.Aggregates(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		wantAggregates := map[string][]uuid.UUID{
			"foobar": {events[0].AggregateID(), events[1].AggregateID()},
		}
		if !reflect.DeepEqual(wantAggregates, aggregates) {
			t.Fatalf("Aggregates should return %v; got %v", wantAggregates, aggregates)
		}

		ids, err := j.AggregatesOf(context.Background(), "foobar")
		if err != nil {
			t.Fatal(err)
		}

		wantIDs := []uuid.UUID{events[0].AggregateID(), events[1].AggregateID()}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "foobar", wantIDs, ids)
		}

		ids, err = j.AggregatesOf(context.Background(), "invalid")
		if err != nil {
			t.Fatal(err)
		}

		if ids != nil {
			t.Fatalf("AggregatesOf(%q) should return %v; got %v", "invalid", nil, ids)
		}

		id, err := j.Aggregate(context.Background(), "foobar")
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "foobar", events[0].AggregateID(), id)
		}

		id, err = j.Aggregate(context.Background(), "invalid")
		if err != nil {
			t.Fatalf("Aggregate(%q) should return %v; got %v", "invalud", uuid.Nil, id)
		}
	}
}

func TestPeriodically_Trigger(t *testing.T) {
	store := memstore.New()
	eventNames := []string{"foo", "bar", "baz"}
	s := project.Periodically(store, time.Hour, eventNames)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan project.Job, 3)
	errs := make(chan error)

	for i := 0; i < 3; i++ {
		sub, err := s.Subscribe(ctx, func(j project.Job) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- j:
				return nil
			}
		})
		if err != nil {
			t.Fatalf("failed to subscribe to Schedule: %v", err)
		}
		go func() {
			for err := range sub {
				select {
				case <-ctx.Done():
					return
				case errs <- err:
				}
			}
		}()
	}

	select {
	case j := <-jobs:
		t.Fatalf("unexpected Job: %v", j)
	case err, ok := <-errs:
		if !ok {
			errs = nil
			break
		}
		t.Fatal(err)
	default:
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	if err := store.Insert(ctx, events...); err != nil {
		t.Fatalf("failed to insert Events: %v", err)
	}

	if err := s.Trigger(ctx); err != nil {
		t.Fatalf("failed to trigger Schedule: %v", err)
	}

	for i := 0; i < 3; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out")
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			t.Fatal(err)
		case j := <-jobs:
			evts, err := j.Events(j.Context())
			if err != nil {
				t.Fatalf("failed to fetch Job Events: %v", err)
			}
			test.AssertEqualEventsUnsorted(t, evts, events)
		}
	}
}

// func TestPeriodically_withLatestEventTime(t *testing.T) {
// 	store := memstore.New()

// 	s := project.Periodically(store, 50*time.Millisecond, []string{"foo", "bar", "baz"})

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	events := []event.Event{
// 		event.New("foo", test.FooEventData{}),
// 		event.New("bar", test.BarEventData{}),
// 		event.New("baz", test.BazEventData{}),
// 	}

// 	if err := store.Insert(ctx, events...); err != nil {
// 		t.Fatalf("failed to insert Events: %v", err)
// 	}

// 	ex := newMockProjection()
// 	ex.applied = events[:1]
// 	ex.PostApplyEvent(events[0])

// 	errs, err := s.Subscribe(
// 		ctx,
// 		func(job project.Job) error {
// 			defer cancel()
// 			if err := job.Apply(job.Context(), ex); err != nil {
// 				return fmt.Errorf("failed to apply Projection Job: %w", err)
// 			}
// 			return nil
// 		},
// 	)
// 	if err != nil {
// 		t.Fatalf("failed to subscribe to Projections: %v", err)
// 	}

// 	for err := range errs {
// 		t.Fatalf("Projection failed: %v", err)
// 	}

// 	if !ex.hasApplied(events...) {
// 		t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
// 	}

// 	if ex.hasDuplicates() {
// 		t.Fatalf("Projection has duplicate applied Events. Does the Projector add query filters to exlcude already applied Events?")
// 	}
// }

func TestFilter(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()

	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"})

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

	errs, err := s.Subscribe(
		ctx,
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

func TestDebounce(t *testing.T) {
	bus := chanbus.New()
	store := memstore.New()

	s := project.Continuously(bus, store, []string{"foo", "bar", "baz"}, project.Debounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := newMockProjection()

	applied := make(chan project.Job)

	errs, err := s.Subscribe(ctx, func(j project.Job) error {
		if err := j.Apply(j.Context(), ex); err != nil {
			return fmt.Errorf("apply Job: %w", err)
		}
		applied <- j
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to Schedule: %v", err)
	}

	events := []event.Event{
		event.New("foo", test.FooEventData{}),
		event.New("bar", test.BarEventData{}),
		event.New("baz", test.BazEventData{}),
	}

	publishError := make(chan error)
	go func() {
		for _, evt := range events {
			if err := bus.Publish(context.Background(), evt); err != nil {
				publishError <- err
			}
			<-time.After(20 * time.Millisecond)
		}
		<-time.After(50 * time.Millisecond)
		cancel()
	}()

	select {
	case err := <-publishError:
		t.Fatalf("failed to publish Event: %v", err)
	case err, ok := <-errs:
		if !ok {
			errs = nil
			break
		}
		t.Fatalf("subscription: %v", err)
	case <-applied:
	}

	select {
	case err := <-publishError:
		t.Fatalf("failed to publish Event: %v", err)
	case err, ok := <-errs:
		if !ok {
			errs = nil
			break
		}
		t.Fatalf("subscription: %v", err)
	case <-applied:
		t.Fatalf("only 1 Job should have been created!")
	case <-time.After(50 * time.Millisecond):
	}

	if !ex.hasApplied(events...) {
		t.Fatalf("applied Events should be %v; got %v", events, ex.applied)
	}

	if ex.hasDuplicates() {
		t.Fatal("projection has duplicate Events applied")
	}
}

type mockProjection struct {
	*project.Progressor

	applied []event.Event
}

func newMockProjection() *mockProjection {
	return &mockProjection{
		Progressor: &project.Progressor{},
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
