package repository_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	mock_event "github.com/modernice/goes/event/mocks"
	etest "github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
)

func TestRepository_Save(t *testing.T) {
	r := repository.New(memstore.New())

	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 3)),
	}

	flushed := make(chan struct{})
	foo := test.NewFoo(aggregateID, test.FlushChangesFunc(func(flush func()) {
		flush()
		close(flushed)
	}))

	if err := foo.TrackChange(events...); err != nil {
		t.Fatalf("expected foo.TrackChange to succeed; got %#v", err)
	}

	if err := r.Save(context.Background(), foo); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("didn't flush after %s", 500*time.Millisecond)
	case _, ok := <-flushed:
		if ok {
			t.Fatalf("flushed should be closed")
		}
	}
}

func TestRepository_Save_rollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := mock_event.NewMockStore(ctrl)
	r := repository.New(mockStore)

	// given 3 events
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 3)),
	}

	a := test.NewFoo(aggregateID)
	if err := a.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	// when the event insert fails
	mockInsertError := errors.New("mock insert error")
	mockStore.EXPECT().Insert(gomock.Any(), events).Return(mockInsertError)

	// it should delete the events
	for _, evt := range events {
		mockStore.EXPECT().Delete(gomock.Any(), evt).Return(nil)
	}

	wantError := &repository.SaveError{
		Aggregate: a,
		Err:       mockInsertError,
		Rollbacks: repository.SaveRollbacks{
			{Event: events[0], Err: nil},
			{Event: events[1], Err: nil},
			{Event: events[2], Err: nil},
		},
	}
	if err := r.Save(context.TODO(), a); !errors.Is(err, wantError) {
		t.Fatalf("r.Save returned wrong error\n\nwant: %#v\n\ngot: %#v\n\n", wantError, err)
	}

	// the aggregate should keep the changes
	etest.AssertEqualEvents(t, events, a.AggregateChanges())
}

func TestRepository_Save_rollbackError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := mock_event.NewMockStore(ctrl)
	r := repository.New(mockStore)

	// given 3 events
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 1)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 2)),
		event.New("foo", etest.FooEventData{}, event.Aggregate("foo", aggregateID, 3)),
	}

	a := test.NewFoo(aggregateID)
	if err := a.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	// when the event insert fails
	mockInsertError := errors.New("mock insert error")
	mockStore.EXPECT().Insert(gomock.Any(), events).Return(mockInsertError)

	// it should delete the events
	var mockDeleteError error
	for i, evt := range events {
		// all except the first rollback fail
		if i != 0 {
			mockDeleteError = errors.New("mock rollback error")
		}
		mockStore.EXPECT().Delete(gomock.Any(), evt).Return(mockDeleteError)
	}

	wantError := &repository.SaveError{
		Aggregate: a,
		Err:       mockInsertError,
		Rollbacks: repository.SaveRollbacks{
			{Event: events[0], Err: nil},
			{Event: events[1], Err: mockDeleteError},
			{Event: events[2], Err: mockDeleteError},
		},
	}
	if err := r.Save(context.TODO(), a); !errors.Is(err, wantError) {
		t.Fatalf("r.Save returned wrong error\n\nwant: %#v\n\ngot: %#v\n\n", wantError, err)
	}

	// the aggregate should keep the changes
	etest.AssertEqualEvents(t, events, a.AggregateChanges())
}

func TestRepository_Fetch(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	if err := org.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	r := repository.New(memstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	var flushed bool
	foo := test.NewFoo(
		aggregateID,
		test.ApplyEventFunc("foo", func(evt event.Event) {
			appliedEvents = append(appliedEvents, evt)
		}),
		test.FlushChangesFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)
	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("expected r.Fetch to succeed; got %#v", err)
	}

	etest.AssertEqualEvents(t, events, appliedEvents)

	if foo.AggregateVersion() != 3 {
		t.Errorf("expected foo.AggregateVersion to return %d; got %d", 3, foo.AggregateVersion())
	}

	if len(foo.AggregateChanges()) != 0 {
		t.Errorf("expected foo to have no changes; got %#v", foo.AggregateChanges())
	}

	if !flushed {
		t.Errorf("expected foo.FlushChanges to have been called")
	}
}

func TestRepository_FetchVersion(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 4)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 5)),
	}

	org := test.NewFoo(aggregateID)
	if err := org.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	r := repository.New(memstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	var flushed bool
	foo := test.NewFoo(
		aggregateID,
		test.ApplyEventFunc("foo", func(evt event.Event) {
			appliedEvents = append(appliedEvents, evt)
		}),
		test.FlushChangesFunc(func(flush func()) {
			flush()
			flushed = true
		}),
	)
	if err := r.FetchVersion(context.Background(), foo, 4); err != nil {
		t.Fatalf("expected r.FetchVersion to succeed; got %#v", err)
	}

	etest.AssertEqualEvents(t, events[:4], appliedEvents)

	if foo.AggregateVersion() != 4 {
		t.Errorf("expected foo.AggregateVersion to return %d; got %d", 4, foo.AggregateVersion())
	}

	if len(foo.AggregateChanges()) != 0 {
		t.Errorf("expected foo to have no changes; got %#v", foo.AggregateChanges())
	}

	if !flushed {
		t.Errorf("expected foo.FlushChanges to have been called")
	}
}

func TestRepository_FetchVersion_zeroOrNegative(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	if err := org.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	r := repository.New(memstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	foo := test.NewFoo(aggregateID, test.ApplyEventFunc("foo", func(evt event.Event) {
		appliedEvents = append(appliedEvents, evt)
	}))

	if err := r.FetchVersion(context.Background(), foo, -2); err != nil {
		t.Fatalf("r.FetchVersion should not return an error; got %#v", err)
	}

	if len(appliedEvents) > 0 {
		t.Errorf("no events should have been applied")
	}

	if foo.AggregateVersion() != 0 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 0, foo.AggregateVersion())
	}

	if err := r.FetchVersion(context.Background(), foo, 0); err != nil {
		t.Fatalf("r.FetchVersion should not return an error; got %#v", err)
	}

	if len(appliedEvents) > 0 {
		t.Errorf("no events should have been applied")
	}

	if foo.AggregateVersion() != 0 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 0, foo.AggregateVersion())
	}
}

func TestRepository_FetchVersion_versionNotReached(t *testing.T) {
	aggregateName := "foo"
	aggregateID := uuid.New()
	events := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 1)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 2)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(aggregateName, aggregateID, 3)),
	}

	org := test.NewFoo(aggregateID)
	if err := org.TrackChange(events...); err != nil {
		t.Fatalf("expected a.TrackChange to succeed; got %#v", err)
	}

	r := repository.New(memstore.New())
	if err := r.Save(context.Background(), org); err != nil {
		t.Fatalf("expected r.Save to succeed; got %#v", err)
	}

	var appliedEvents []event.Event
	foo := test.NewFoo(aggregateID, test.ApplyEventFunc("foo", func(evt event.Event) {
		appliedEvents = append(appliedEvents, evt)
	}))

	if err := r.FetchVersion(context.Background(), foo, 9999); err != nil {
		t.Fatalf("r.FetchVersion should not return an error; got %#v", err)
	}

	if len(appliedEvents) != 3 {
		t.Errorf("no events should have been applied")
	}

	if foo.AggregateVersion() != 3 {
		t.Errorf("foo.AggregateVersion should return %d; got %d", 3, foo.AggregateVersion())
	}
}

func TestRepository_Delete(t *testing.T) {
	foo := test.NewFoo(uuid.New())
	changes := []event.Event{
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			1,
		)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			2,
		)),
		event.New("foo", etest.FooEventData{A: "foo"}, event.Aggregate(
			foo.AggregateName(),
			foo.AggregateID(),
			3,
		)),
	}

	s := memstore.New(changes...)
	r := repository.New(s)

	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("r.Fetch should not fail: %#v", err)
	}

	if v := foo.AggregateVersion(); v != 3 {
		t.Fatalf("foo.AggregateVersion should return %d; got %d", 3, v)
	}

	if err := r.Delete(context.Background(), foo); err != nil {
		t.Fatalf("r.Delete should not fail: %#v", err)
	}

	foo = test.NewFoo(foo.AggregateID())
	if err := r.Fetch(context.Background(), foo); err != nil {
		t.Fatalf("r.Fetch should not fail: %#v", err)
	}

	if v := foo.AggregateVersion(); v != 0 {
		t.Fatalf("foo.AggregateVersion should return %d; got %d", 0, v)
	}
}

func TestRepository_Query_name(t *testing.T) {
	t.Skip()

	foos, _ := xaggregate.Make(3, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(3, xaggregate.Name("bar"))
	bazs, _ := xaggregate.Make(3, xaggregate.Name("baz"))
	as := append(foos, append(bars, bazs...)...)

	events := xevent.Make("foo", etest.FooEventData{}, 10, xevent.ForAggregate(as...))
	for _, a := range as {
		aevents := xevent.FilterAggregate(events, a)
		for _, evt := range aevents {
			a.ApplyEvent(evt)
		}
		a.TrackChange(aevents...)
	}

	s := memstore.New()
	r := repository.New(s)

	for _, a := range as {
		if err := r.Save(context.Background(), a); err != nil {
			t.Fatalf("r.Save should not fail: %v", err)
		}
	}

	result, err := runQuery(r, query.New(query.Name("foo")))
	if err != nil {
		t.Fatal(err)
	}

	foos = aggregate.Sort(foos, aggregate.SortID, aggregate.SortAsc)
	result = aggregate.Sort(result, aggregate.SortID, aggregate.SortAsc)

	if !reflect.DeepEqual(result, foos) {
		t.Fatalf("query returned the wrong aggregates\n\nwant: %#v\n\ngot: %#v\n\n", foos, result)
	}
}

func runQuery(r aggregate.Repository, q query.Query) ([]aggregate.Aggregate, error) {
	cur, err := r.Query(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return stream.All(context.Background(), cur)
}
