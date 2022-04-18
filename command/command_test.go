package command_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

type mockPayload struct {
	A bool
	B string
}

func TestNew(t *testing.T) {
	pl := mockPayload{
		A: true,
		B: "bar",
	}
	cmd := command.New("foo", pl)

	var _ uuid.UUID = cmd.ID()

	if cmd.ID() == uuid.Nil {
		t.Errorf("cmd.ID should return a non-zero UUID; got %s", cmd.ID())
	}

	if cmd.Name() != "foo" {
		t.Errorf("cmd.Name should return %q; got %q", "foo", cmd.Name())
	}

	if cmd.Payload() != pl {
		t.Errorf("cmd.Payload should return %#v; got %#v", pl, cmd.Payload())
	}
}

func TestID(t *testing.T) {
	id := uuid.New()
	cmd := command.New[any]("foo", mockPayload{}, command.ID(id))

	if cmd.ID() != id {
		t.Errorf("ID Option did not apply. want=%s got=%s", id, cmd.ID())
	}
}

func TestAggregateName(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	cmd := command.New[any](
		"foo",
		mockPayload{},
		command.Aggregate(a.AggregateName(), a.AggregateID()),
	)

	id, name := cmd.Aggregate().Split()

	if name != a.AggregateName() {
		t.Fatalf(
			"cmd.AggregateName should return %q; got %q",
			a.AggregateName(),
			name,
		)
	}

	if id != a.AggregateID() {
		t.Fatalf(
			"cmd.AggregateID should return %q; got %q",
			a.AggregateID(),
			id,
		)
	}
}
