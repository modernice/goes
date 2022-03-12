package auth_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/test"
)

func TestNewUUIDActor(t *testing.T) {
	test.NewAggregate(t, auth.NewUUIDActor, auth.ActorAggregate)

	a := auth.NewUUIDActor(uuid.New())

	if a.ActorID() != a.AggregateID() {
		t.Fatalf("ActorID() should return the aggregate id %q for a UUID-Actor; got %q", a.AggregateID(), a.ActorID())
	}

	if a.ActorKind() != "uuid" {
		t.Fatalf("ActorKind() should return %q; got %q", "uuid", a.ActorKind())
	}
}

func TestNewStringActor(t *testing.T) {
	test.NewAggregate(t, auth.NewStringActor, auth.ActorAggregate)

	a := auth.NewStringActor(uuid.New())

	if a.ActorID() != nil {
		t.Fatalf("ActorID() should initially return nil for a StringActor; got %q", a.ActorID())
	}

	if a.ActorKind() != "string" {
		t.Fatalf("ActorKind() should return %q; got %q", "string", a.ActorKind())
	}
}

func TestActor_Grant_ErrInvalidRef(t *testing.T) {
	a := auth.NewUUIDActor(uuid.New())

	ref := aggregate.Ref{
		Name: "",
		ID:   uuid.New(),
	}

	if err := a.Grant(ref, "view"); !errors.Is(err, auth.ErrInvalidRef) {
		t.Fatalf("Grant() should fail with %q if the provided Ref has an empty name; got %q", auth.ErrInvalidRef, err)
	}

	test.NoChange(t, a, auth.PermissionGranted)
}

func TestActor_Revoke_ErrInvalidRef(t *testing.T) {
	a := auth.NewUUIDActor(uuid.New())

	ref := aggregate.Ref{
		Name: "",
		ID:   uuid.New(),
	}

	if err := a.Revoke(ref, "view"); !errors.Is(err, auth.ErrInvalidRef) {
		t.Fatalf("Revoke() should fail with %q if the provided Ref has an empty name; got %q", auth.ErrInvalidRef, err)
	}

	test.NoChange(t, a, auth.PermissionRevoked)
}

func TestActor_Grant_Revoke(t *testing.T) {
	a := auth.NewUUIDActor(uuid.New())

	actions := []string{"view", "update"}
	ref := aggregate.Ref{
		Name: "foo",
		ID:   uuid.New(),
	}

	for _, action := range actions {
		if a.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return false before granting permission", action)
		}
	}

	a.Grant(ref, actions...)

	for _, action := range actions {
		if !a.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return true after granting permission", action)
		}
	}

	test.Change(t, a, auth.PermissionGranted, test.EventData(auth.PermissionGrantedData{
		Actions:   actions,
		Aggregate: ref,
	}))

	a.Revoke(ref, actions...)

	for _, action := range actions {
		if a.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return false after revoking permission", action)
		}
	}

	test.Change(t, a, auth.PermissionRevoked, test.EventData(auth.PermissionRevokedData{
		Actions:   actions,
		Aggregate: ref,
	}))
}

func TestActor_Grant_Revoke_ErrMissingActorID(t *testing.T) {
	a := auth.NewStringActor(uuid.New())

	ref := aggregate.Ref{
		Name: "foo",
		ID:   uuid.New(),
	}

	if err := a.Grant(ref, "view"); !errors.Is(err, auth.ErrMissingActorID) {
		t.Fatalf("Grant() should fail with %q if the actor is unidentified; got %q", auth.ErrMissingActorID, err)
	}

	if err := a.Revoke(ref, "view"); !errors.Is(err, auth.ErrMissingActorID) {
		t.Fatalf("Revoke() should fail with %q if the actor is unidentified; got %q", auth.ErrMissingActorID, err)
	}

	test.NoChange(t, a, auth.PermissionGranted)
	test.NoChange(t, a, auth.PermissionRevoked)
}

func TestActor_Identify_UUID(t *testing.T) {
	a := auth.NewUUIDActor(uuid.New())

	if err := a.Identify(uuid.New()); err != nil {
		t.Fatalf("Identify() should not return an error, even if provided a UUID")
	}

	test.NoChange(t, a, auth.ActorIdentified)
}

func TestActor_Identify_string_ErrIDType(t *testing.T) {
	a := auth.NewStringActor(uuid.New())

	var id int
	if err := a.Identify(id); !errors.Is(err, auth.ErrIDType) {
		t.Fatalf("Identify() of a string-Actor should not return a %T error if provided an id that is not a string; got %q", auth.ErrIDType, err)
	}

	test.NoChange(t, a, auth.ActorIdentified)
}

func TestActor_Identify_string(t *testing.T) {
	a := auth.NewStringActor(uuid.New())

	id := "foo"
	if err := a.Identify(id); err != nil {
		t.Fatalf("Identify() of a string-Actor should not return a %T error if provided an id that is not a string; got %q", auth.ErrIDType, err)
	}

	if a.ActorID() != id {
		t.Fatalf("ActorID() should return %q; got %q", id, a.ActorID())
	}

	test.Change(t, a, auth.ActorIdentified, test.EventData(auth.ActorIdentifiedData(id)))
}
