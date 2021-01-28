package factory_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/factory"
)

func TestFactory_Make_unknownName(t *testing.T) {
	f := factory.New()
	a, err := f.Make("foo", uuid.New())
	if !errors.Is(err, factory.ErrUnknownName) {
		t.Errorf("f.Make should return error %#v; got %#v", factory.ErrUnknownName, err)
	}

	if a != nil {
		t.Errorf("f.Make should return no aggregate; got %#v", a)
	}
}

func TestFactory_Make(t *testing.T) {
	name := "foo"
	id := uuid.New()
	f := factory.New(factory.For(name, func(id uuid.UUID) aggregate.Aggregate {
		return aggregate.New(name, id)
	}))

	a, err := f.Make(name, id)
	if err != nil {
		t.Errorf("f.Make should return no error; got %#v", err)
	}

	if a == nil {
		t.Errorf("f.Make should return an aggregate; got %#v", a)
	}

	if a.AggregateName() != name {
		t.Errorf("f.Make returned an aggregate with a wrong name. want=%q got=%q", name, a.AggregateName())
	}

	if a.AggregateID() != id {
		t.Errorf("f.Make returned an aggregate with a wrong id. want=%s got=%s", id, a.AggregateID())
	}
}
