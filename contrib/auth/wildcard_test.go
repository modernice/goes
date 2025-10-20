package auth_test

import (
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/contrib/auth"
	"github.com/modernice/goes/projection"
)

// WildcardTest represents a test case for checking if an Allower allows or
// disallows an action on a specified aggregate reference using wildcard
// permissions.
type WildcardTest struct {
	name          string
	wildcard      aggregate.Ref
	actions       []string
	wantAllows    []WildcardAllowTest
	wantDisallows []WildcardAllowTest
}

// WildcardAllowTest represents a test case for checking if an [Allower] allows
// or disallows an action on a specified aggregate reference.
type WildcardAllowTest struct {
	ref    aggregate.Ref
	action string
}

func TestWildcards(t *testing.T) {
	id := uuid.New()

	tests := []WildcardTest{
		{
			name: "all aggregates, all ids, all actions",
			wildcard: aggregate.Ref{
				Name: "*",
				ID:   uuid.Nil,
			},
			actions: []string{"*"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "baz", ID: uuid.New()},
					action: "baz",
				},
			},
		},
		{
			name: "all aggregates, all ids, single action",
			wildcard: aggregate.Ref{
				Name: "*",
				ID:   uuid.Nil,
			},
			actions: []string{"foo"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: uuid.New()},
					action: "foo",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "bar",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: uuid.New()},
					action: "baz",
				},
			},
		},
		{
			name: "all aggregates, single id, all actions",
			wildcard: aggregate.Ref{
				Name: "*",
				ID:   id,
			},
			actions: []string{"*"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: id},
					action: "bar",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: uuid.New()},
					action: "bar",
				},
			},
		},
		{
			name: "all aggregates, single id, single action",
			wildcard: aggregate.Ref{
				Name: "*",
				ID:   id,
			},
			actions: []string{"foo"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: id},
					action: "foo",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "bar",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: id},
					action: "baz",
				},
			},
		},
		{
			name: "single aggregate, all ids, all actions",
			wildcard: aggregate.Ref{
				Name: "foo",
				ID:   uuid.Nil,
			},
			actions: []string{"*"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "bar",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "bar", ID: uuid.New()},
					action: "foo",
				},
			},
		},
		{
			name: "single aggregate, single id, all actions",
			wildcard: aggregate.Ref{
				Name: "foo",
				ID:   id,
			},
			actions: []string{"*"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "bar",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "bar", ID: id},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
			},
		},
		{
			name: "single aggregate, single id, single action",
			wildcard: aggregate.Ref{
				Name: "foo",
				ID:   id,
			},
			actions: []string{"foo"},
			wantAllows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "foo",
				},
			},
			wantDisallows: []WildcardAllowTest{
				{
					ref:    aggregate.Ref{Name: "foo", ID: uuid.New()},
					action: "foo",
				},
				{
					ref:    aggregate.Ref{Name: "foo", ID: id},
					action: "bar",
				},
				{
					ref:    aggregate.Ref{Name: "bar", ID: id},
					action: "foo",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("Actor", func(t *testing.T) {
				runWildcardTest(t, tt, func() *auth.Actor {
					return auth.NewUUIDActor(uuid.New())
				})
			})

			t.Run("Role", func(t *testing.T) {
				runWildcardTest(t, tt, func() *auth.Role {
					r := auth.NewRole(uuid.New())
					r.Identify("admin")
					return r
				})
			})

			t.Run("Permissions", func(t *testing.T) {
				actor := auth.NewUUIDActor(uuid.New())
				role := auth.NewRole(uuid.New())
				role.Identify("admin")
				role.Add(actor.AggregateID())

				switch rand.Intn(2) {
				case 0:
					role.Grant(tt.wildcard, tt.actions...)
				case 1:
					actor.Grant(tt.wildcard, tt.actions...)
				}

				events := append(role.AggregateChanges(), actor.AggregateChanges()...)

				perms := auth.PermissionsOf(actor.AggregateID())
				projection.Apply(perms, events)

				runWildcardTestWithoutGrant(t, tt, func() *auth.Permissions {
					return perms
				})
			})
		})
	}
}

func runWildcardTest[A AllowerGranter](t *testing.T, tt WildcardTest, makeActor func() A) {
	a := makeActor()

	if err := a.Grant(tt.wildcard, tt.actions...); err != nil {
		t.Fatalf("Grant() failed with %q", err)
	}

	for _, allow := range tt.wantAllows {
		if !a.Allows(allow.action, allow.ref) {
			t.Fatalf("%q action on %s aggregate should be allowed", allow.action, allow.ref)
		}
	}

	for _, disallow := range tt.wantDisallows {
		if !a.Disallows(disallow.action, disallow.ref) {
			t.Fatalf("%q action on %s aggregate should be disallowed", disallow.action, disallow.ref)
		}
	}
}

func runWildcardTestWithoutGrant[A Allower](t *testing.T, tt WildcardTest, makeActor func() A) {
	a := makeActor()

	for _, allow := range tt.wantAllows {
		if !a.Allows(allow.action, allow.ref) {
			t.Fatalf("%q action on %s aggregate should be allowed", allow.action, allow.ref)
		}
	}

	for _, disallow := range tt.wantDisallows {
		if !a.Disallows(disallow.action, disallow.ref) {
			t.Fatalf("%q action on %s aggregate should be disallowed", disallow.action, disallow.ref)
		}
	}
}

// Allower is an interface that defines methods for checking if an action is
// allowed or disallowed for a specific aggregate reference. It provides Allows
// and Disallows methods, which return true if the action is allowed or
// disallowed, respectively, for the given aggregate reference.
type Allower interface {
	// Allows checks if the given action is allowed on the specified aggregate
	// reference. Returns true if the action is allowed, otherwise false.
	Allows(action string, ref aggregate.Ref) bool

	// Disallows checks if the given action is disallowed on the specified
	// aggregate.Ref by the Allower. Returns true if the action is disallowed, false
	// otherwise.
	Disallows(action string, ref aggregate.Ref) bool
}

// AllowerGranter is an interface that combines the capabilities of allowing and
// disallowing actions on specified [aggregate.Ref]s and granting permissions
// for those actions.
type AllowerGranter interface {
	Allower
	// Grant allows specified actions on the provided aggregate reference for the
	// implementing type (such as an Actor or Role). If the wildcard aggregate
	// reference is used, it grants the actions on all aggregate references.
	Grant(ref aggregate.Ref, actions ...string) error
}
