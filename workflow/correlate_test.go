package workflow_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/workflow"
)

func TestByAggregateID(t *testing.T) {
	aggregateID := uuid.New()
	evt := event.New("test.placed", orderPlacedData{}, event.Aggregate(aggregateID, "order", 1))

	id, ok := workflow.ByAggregateID(evt)
	if !ok {
		t.Fatal("expected event with aggregate to correlate")
	}
	if id != aggregateID {
		t.Fatalf("expected workflow id %s; got %s", aggregateID, id)
	}

	if _, ok := workflow.ByAggregateID(event.New("test.placed", orderPlacedData{})); ok {
		t.Fatal("expected event without aggregate to be ignored")
	}
}

func TestByKey(t *testing.T) {
	type signedUp struct{ Email string }

	namespace := uuid.New()
	correlate := workflow.ByKey(namespace, func(d signedUp) string { return d.Email })

	evt := func(email string) event.Of[signedUp] {
		return event.New("test.signed_up", signedUp{Email: email})
	}

	alice1, ok := correlate(evt("alice@example.com"))
	if !ok {
		t.Fatal("expected event with key to correlate")
	}

	alice2, _ := correlate(evt("alice@example.com"))
	if alice1 != alice2 {
		t.Fatalf("expected deterministic ids for the same key; got %s and %s", alice1, alice2)
	}

	bob, _ := correlate(evt("bob@example.com"))
	if bob == alice1 {
		t.Fatal("expected different keys to yield different workflow ids")
	}

	if _, ok := correlate(evt("")); ok {
		t.Fatal("expected empty key to be ignored")
	}

	other := workflow.ByKey(uuid.New(), func(d signedUp) string { return d.Email })
	if otherAlice, _ := other(evt("alice@example.com")); otherAlice == alice1 {
		t.Fatal("expected different namespaces to yield different workflow ids")
	}
}
