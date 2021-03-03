package project_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/project"
)

func TestNew(t *testing.T) {
	name := "foo"
	id := uuid.New()
	p := project.New(name, id)

	if p.AggregateName() != name {
		t.Errorf("AggregateName should return %q; got %q", name, p.AggregateName())
	}

	if p.AggregateID() != id {
		t.Errorf("AggregateID should return %q; got %q", id, p.AggregateID())
	}
}
