package auth_test

import (
	"testing"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/projection"
)

func TestPermissions(t *testing.T) {
	ref := aggregate.Ref{
		Name: "foo",
		ID:   internal.NewUUID(),
	}
	actions := []string{"foo", "bar", "baz"}

	actor := auth.NewUUIDActor(internal.NewUUID())
	actor.Grant(ref, actions...)

	perms := auth.PermissionsOf(actor.AggregateID())

	if perms.Allows("foo", ref) {
		t.Fatalf("Permissions should not allow %q action before being projected", "foo")
	}

	projection.Apply(perms, actor.AggregateChanges())

	if !perms.Allows("foo", ref) {
		t.Fatalf("Permissions should allow %q action after being projected", "foo")
	}
}

func TestPermissions_ofRole(t *testing.T) {
	ref := aggregate.Ref{
		Name: "foo",
		ID:   internal.NewUUID(),
	}
	actions := []string{"foo", "bar", "baz"}

	actor := auth.NewUUIDActor(internal.NewUUID())

	role := auth.NewRole(internal.NewUUID())
	role.Identify("admin")
	role.Add(actor.ID)
	role.Grant(ref, actions...)

	perms := auth.PermissionsOf(actor.AggregateID())

	if perms.Allows("foo", ref) {
		t.Fatalf("Permissions should not allow %q action before being projected", "foo")
	}

	projection.Apply(perms, role.AggregateChanges())

	if !perms.Allows("foo", ref) {
		t.Fatalf("Permissions should allow %q action after being projected", "foo")
	}
}

func TestPermissions_cases(t *testing.T) {
	tests := []struct {
		name           string
		grantActor     []string
		revokeActor    []string
		grantRole      []string
		revokeRole     []string
		wantAllowed    []string
		wantDisallowed []string
	}{
		{
			name:           "no permissions",
			wantDisallowed: []string{"foo", "bar", "baz"},
		},
		{
			name:        "actor has permission",
			grantActor:  []string{"foo"},
			wantAllowed: []string{"foo"},
		},
		{
			name:        "role has permission",
			grantRole:   []string{"foo"},
			wantAllowed: []string{"foo"},
		},
		{
			name:        "actor and role have permission",
			grantActor:  []string{"foo"},
			grantRole:   []string{"foo"},
			wantAllowed: []string{"foo"},
		},
		{
			name:        "role has permission, actor was revoked permission",
			grantRole:   []string{"foo"},
			grantActor:  []string{"foo"},
			revokeActor: []string{"foo"},
			wantAllowed: []string{"foo"},
		},
		{
			name:        "role was revoked permission, but actor has permission",
			grantRole:   []string{"foo"},
			revokeRole:  []string{"foo"},
			grantActor:  []string{"foo"},
			wantAllowed: []string{"foo"},
		},
		{
			name:        "actor was granted permission, role was granted permission, actor was revoked permission",
			grantActor:  []string{"foo"},
			grantRole:   []string{"foo"},
			revokeActor: []string{"foo"},
			wantAllowed: []string{"foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := aggregate.Ref{
				Name: "foo",
				ID:   internal.NewUUID(),
			}

			actor := auth.NewUUIDActor(internal.NewUUID())
			role := auth.NewRole(internal.NewUUID())

			actor.Grant(ref, tt.grantActor...)
			actor.Revoke(ref, tt.revokeActor...)

			role.Identify("admin")
			role.Add(actor.ID)
			role.Grant(ref, tt.grantRole...)
			role.Revoke(ref, tt.revokeRole...)

			events := append(actor.AggregateChanges(), role.AggregateChanges()...)

			perms := auth.PermissionsOf(actor.AggregateID())
			projection.Apply(perms, events)

			for _, action := range tt.wantAllowed {
				if !perms.Allows(action, ref) {
					t.Fatalf("%q action should be allowed", action)
				}
			}

			for _, action := range tt.wantDisallowed {
				if !perms.Disallows(action, ref) {
					t.Fatalf("%q action should be disallowed", action)
				}
			}
		})
	}
}
