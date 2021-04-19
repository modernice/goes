package project_test

import (
	"reflect"
	"testing"

	"github.com/modernice/goes/aggregate/project"
)

func TestEvents(t *testing.T) {
	names := []string{"foo", "bar", "baz"}
	p := project.Events(names...)

	q := p.EventQuery()
	if q == nil {
		t.Fatalf("EventQuery should return a Query; got %v\n", q)
	}

	if qnames := q.Names(); !reflect.DeepEqual(names, qnames) {
		t.Fatalf("EventQuery.Names should return %v; got %v\n", names, qnames)
	}
}

func TestAggregates(t *testing.T) {
	names := []string{"foo", "bar", "baz"}
	p := project.Aggregates(names...)

	q := p.EventQuery()
	if q == nil {
		t.Fatalf("EventQuery should return a Query; got %v\n", q)
	}

	if qnames := q.AggregateNames(); !reflect.DeepEqual(names, qnames) {
		t.Fatalf("EventQuery.AggregateNames should return %v; got %v\n", names, qnames)
	}
}
