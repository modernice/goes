package stream_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/stream"
)

func TestStream(t *testing.T) {
	aggregates := makeAggregates()
	cur := stream.New(aggregates...)

	var cursorAggregates []aggregate.Aggregate
	for cur.Next(context.Background()) {
		cursorAggregates = append(cursorAggregates, cur.Aggregate())
	}

	if err := cur.Err(); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Err to return %#v; got %#v", error(nil), err))
	}

	if !reflect.DeepEqual(aggregates, cursorAggregates) {
		t.Errorf(
			"expected cursor aggregates to equal original aggregates\noriginal: %#v\n\ngot: %#v\n\n",
			aggregates,
			cursorAggregates,
		)
	}

	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %v", err))
	}
}

func TestStream_Next_closed(t *testing.T) {
	cur := stream.New(makeAggregates()...)
	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %v", err))
	}

	if ok := cur.Next(context.Background()); ok {
		t.Errorf("expected cur.Next to return %t; got %t", false, ok)
	}

	if err := cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Error(fmt.Errorf("expected cur.Err to return %#v; got %#v", stream.ErrClosed, err))
	}

	if evt := cur.Aggregate(); evt != nil {
		t.Error(fmt.Errorf("expected cur.Aggregate to return %#v; got %#v", aggregate.Aggregate(nil), evt))
	}
}

func TestAll(t *testing.T) {
	aggregates := makeAggregates()
	cur := stream.New(aggregates...)

	all, err := stream.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected stream.All not to return an error; got %#v", err))
	}

	if !reflect.DeepEqual(aggregates, all) {
		t.Errorf(
			"expected cursor aggregates to equal original aggregates\noriginal: %#v\n\ngot: %#v\n\n",
			aggregates,
			all,
		)
	}

	if ok := cur.Next(context.Background()); ok {
		t.Errorf("expected cur.Next to return %t; got %t", false, ok)
	}

	if err = cur.Err(); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("expected cur.Err to return %#v; got %#v", stream.ErrClosed, err)
	}
}

func TestAll_partial(t *testing.T) {
	aggregates := makeAggregates()
	cur := stream.New(aggregates...)
	if !cur.Next(context.Background()) {
		t.Fatal(fmt.Errorf("cur.Next: %w", cur.Err()))
	}

	all, err := stream.All(context.Background(), cur)
	if err != nil {
		t.Fatal(fmt.Errorf("expected stream.All not to return an error; got %v", err))
	}

	if !reflect.DeepEqual(aggregates[1:], all) {
		t.Errorf(
			"expected cursor aggregates to equal original aggregates\noriginal: %#v\n\ngot: %#v\n\n",
			aggregates[1:],
			all,
		)
	}
}

func TestAll_closed(t *testing.T) {
	aggregates := makeAggregates()
	cur := stream.New(aggregates...)

	if err := cur.Close(context.Background()); err != nil {
		t.Fatal(fmt.Errorf("expected cur.Close not to return an error; got %#v", err))
	}

	if _, err := stream.All(context.Background(), cur); !errors.Is(err, stream.ErrClosed) {
		t.Errorf("expected stream.All to return %#v; got %#v", stream.ErrClosed, err)
	}
}

func makeAggregates() []aggregate.Aggregate {
	return []aggregate.Aggregate{
		aggregate.New("foo", uuid.New()),
		aggregate.New("bar", uuid.New()),
		aggregate.New("baz", uuid.New()),
	}
}
