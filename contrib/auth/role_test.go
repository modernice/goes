package auth_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/test"
)

func TestNewRole(t *testing.T) {
	test.NewAggregate(t, auth.NewRole, auth.RoleAggregate)
}

func TestRole_Identify(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())

	if err := r.Identify("admin"); err != nil {
		t.Fatalf("Identify() failed with %q", err)
	}

	if r.Name() != "admin" {
		t.Fatalf("Name() should return the role name %q; got %q", "admin", r.Name())
	}

	test.Change(t, r, auth.RoleIdentified, test.EventData(auth.RoleIdentifiedData("admin")))
}

func TestRole_Identify_ErrEmptyName(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())

	if err := r.Identify(""); !errors.Is(err, auth.ErrEmptyName) {
		t.Fatalf("Identify() should fail with %q if provided an empty name; got %q", auth.ErrEmptyName, err)
	}

	test.NoChange(t, r, auth.RoleIdentified)
}

func TestRole_Grant_ErrInvalidRef(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())
	r.Identify("admin")

	ref := aggregate.Ref{
		Name: "",
		ID:   internal.NewUUID(),
	}

	if err := r.Grant(ref, "view"); !errors.Is(err, auth.ErrInvalidRef) {
		t.Fatalf("Grant() should fail with %q if the provided Ref has an empty name; got %q", auth.ErrInvalidRef, err)
	}

	test.NoChange(t, r, auth.PermissionGranted)
}

func TestRole_Revoke_ErrInvalidRef(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())
	r.Identify("admin")

	ref := aggregate.Ref{
		Name: "",
		ID:   internal.NewUUID(),
	}

	if err := r.Revoke(ref, "view"); !errors.Is(err, auth.ErrInvalidRef) {
		t.Fatalf("Revoke() should fail with %q if the provided Ref has an empty name; got %q", auth.ErrInvalidRef, err)
	}

	test.NoChange(t, r, auth.PermissionRevoked)
}

func TestRole_Grant_Revoke(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())
	r.Identify("admin")

	actions := []string{"view", "update"}
	ref := aggregate.Ref{
		Name: "foo",
		ID:   internal.NewUUID(),
	}

	for _, action := range actions {
		if r.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return false before granting permission", action)
		}
	}

	r.Grant(ref, actions...)

	for _, action := range actions {
		if !r.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return true after granting permission", action)
		}
	}

	test.Change(t, r, auth.PermissionGranted, test.EventData(auth.PermissionGrantedData{
		Actions:   actions,
		Aggregate: ref,
	}))

	r.Revoke(ref, actions...)

	for _, action := range actions {
		if r.Allows(action, ref) {
			t.Fatalf("Allows(%q) should return false after revoking permission", action)
		}
	}

	test.Change(t, r, auth.PermissionRevoked, test.EventData(auth.PermissionRevokedData{
		Actions:   actions,
		Aggregate: ref,
	}))
}

func TestRole_Grant_Revoke_Add_Remove_ErrMissingRoleName(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())

	actions := []string{"view", "update"}
	ref := aggregate.Ref{
		Name: "foo",
		ID:   internal.NewUUID(),
	}

	if err := r.Grant(ref, actions...); !errors.Is(err, auth.ErrMissingRoleName) {
		t.Fatalf("Grant() should fail with %q if called before the role was identified; got %q", auth.ErrMissingRoleName, err)
	}

	test.NoChange(t, r, auth.PermissionGranted)

	if err := r.Revoke(ref, actions...); !errors.Is(err, auth.ErrMissingRoleName) {
		t.Fatalf("Revoke() should fail with %q if called before the role was identified; got %q", auth.ErrMissingRoleName, err)
	}

	test.NoChange(t, r, auth.PermissionRevoked)

	if err := r.Add(internal.NewUUID()); !errors.Is(err, auth.ErrMissingRoleName) {
		t.Fatalf("Add() should fail with %q if called before the role was identified; got %q", auth.ErrMissingRoleName, err)
	}

	test.NoChange(t, r, auth.RoleGiven)

	if err := r.Remove(internal.NewUUID()); !errors.Is(err, auth.ErrMissingRoleName) {
		t.Fatalf("Remove() should fail with %q if called before the role was identified; got %q", auth.ErrMissingRoleName, err)
	}

	test.NoChange(t, r, auth.RoleRemoved)
}

func TestRole_Add_Remove(t *testing.T) {
	r := auth.NewRole(internal.NewUUID())
	r.Identify("admin")

	actors := []uuid.UUID{internal.NewUUID(), internal.NewUUID()}

	for _, actor := range actors {
		if r.IsMember(actor) {
			t.Fatalf("Actor should not be a member of the role before being added")
		}
	}

	r.Add(actors...)

	for _, actor := range actors {
		if !r.IsMember(actor) {
			t.Fatalf("Actor should be a member of the role after being added")
		}
	}

	test.Change(t, r, auth.RoleGiven, test.EventData(actors))

	r.Remove(actors...)

	for _, actor := range actors {
		if r.IsMember(actor) {
			t.Fatalf("Actor should not be a member of the role after being removed")
		}
	}

	test.Change(t, r, auth.RoleRemoved, test.EventData(actors))
}
