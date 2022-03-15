package auth

import (
	"errors"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/slice"
)

// RoleAggregate is the name of the Role aggregate.
const RoleAggregate = "goes.contrib.auth.role"

var (
	// ErrEmptyName is returned when trying to create a role with an empty name.
	ErrEmptyName = errors.New("empty name")

	// ErrMissingRoleName is returned when trying to grant or revoke permissions
	// to or from a role before giving the role a name.
	ErrMissingRoleName = errors.New("missing role name")
)

// Role represents a named authorization role. Like actors, roles can be granted
// permissions to perform actions on specific aggregates. Actors can be added to
// and removed from roles. Actors that are members of a role inherit the role's
// permissions. A role must be given a name before it can be granted permissions.
//
// Example: "admin" role
//	role := auth.NewRole(uuid.New())
//	role.Identify("admin")
//	role.Grant(aggregate.Ref{Name: "foo", ID: uuid.UUID{...}}, "read", "write")
type Role struct {
	*aggregate.Base

	name    string
	members []uuid.UUID
	Actions
}

// NewRole returns the role with the given id.
func NewRole(id uuid.UUID) *Role {
	r := &Role{
		Base:    aggregate.New(RoleAggregate, id),
		Actions: make(Actions),
	}

	event.ApplyWith(r, r.identify, RoleIdentified)
	event.ApplyWith(r, r.Actions.granted, PermissionGranted)
	event.ApplyWith(r, r.Actions.revoked, PermissionRevoked)
	event.ApplyWith(r, r.add, RoleGiven)
	event.ApplyWith(r, r.remove, RoleRemoved)

	return r
}

// Name returns the name of the role.
func (r *Role) Name() string {
	return r.name
}

// Identify identifies the role with the given name, which must must not be empty.
// Identify must be called before r.Grant() or r.Revoke() is called; otherwise
// these methods will return an error that satisfies errors.Is(err, ErrMissingRoleName).
func (r *Role) Identify(name string) error {
	if name == "" {
		return ErrEmptyName
	}
	aggregate.Next(r, RoleIdentified, RoleIdentifiedData(name))
	return nil
}

func (r *Role) identify(evt event.Of[RoleIdentifiedData]) {
	r.name = string(evt.Data())
}

// Allows returns whether the role has the permission to perform the given action.
func (r *Role) Allows(action string, ref aggregate.Ref) bool {
	return r.allows(action, ref)
}

// Disallows returns whether the role does not have the permission to perform
// the given action.
func (r *Role) Disallows(action string, ref aggregate.Ref) bool {
	return !r.allows(action, ref)
}

// Grant grants the role the permission to perform the given actions on the given aggregate.
//
// Wildcards
//
// Grant supports wildcards in the aggregate reference and actions.
// Pass in a "*" where a string is expected or uuid.Nil where a UUID is expected
// to match all values.
//
// Example – Grant "view" permission on all aggregates with a specific id:
//	var id uuid.UUID
//	role.Grant(aggregate.Ref{Name: "*", ID: id}, "view")
//
// Example – Grant "view" permission on "foo" aggregates with any id:
//	role.Grant(aggregate.Ref{Name: "foo", ID: uuid.Nil}, "view")
//
// Example – Grant "view" permission on all aggregates:
//	role.Grant(aggregate.Ref{Name: "*", ID: uuid.Nil}, "view")
//
// Example – Grant all permissions on all aggregates:
//	role.Grant(aggregate.Ref{Name: "*", ID: uuid.Nil}, "*")
func (r *Role) Grant(ref aggregate.Ref, actions ...string) error {
	if err := r.checkName(); err != nil {
		return err
	}

	if err := validateRef(ref); err != nil {
		return err
	}

	actions = r.missingActions(ref, actions)

	if len(actions) == 0 {
		return nil
	}

	aggregate.Next(r, PermissionGranted, PermissionGrantedData{
		Aggregate: ref,
		Actions:   actions,
	})

	return nil
}

func (r *Role) checkName() error {
	if r.name == "" {
		return ErrMissingRoleName
	}
	return nil
}

// Revoke revokes the role's permission to perform the given actions on the given aggregate.
//
// Wildcards
//
// Revoke supports wildcards in the aggregate reference and actions.
// Pass in a "*" where a string is expected or uuid.Nil where a UUID is expected
// to match all values.
//
// Example – Revoke "view" permission on all aggregates with a specific id:
//	var id uuid.UUID
//	role.Revoke(aggregate.Ref{Name: "*", ID: id}, "view")
//
// Example – Revoke "view" permission on "foo" aggregates with any id:
//	role.Revoke(aggregate.Ref{Name: "foo", ID: uuid.Nil}, "view")
//
// Example – Revoke "view" permission on all aggregates:
//	role.Revoke(aggregate.Ref{Name: "*", ID: uuid.Nil}, "view")
//
// Example – Revoke all permissions on all aggregates:
//	role.Revoke(aggregate.Ref{Name: "*", ID: uuid.Nil}, "*")
func (r *Role) Revoke(ref aggregate.Ref, actions ...string) error {
	if err := r.checkName(); err != nil {
		return err
	}

	if err := validateRef(ref); err != nil {
		return err
	}

	actions = r.grantedActions(ref, actions)

	if len(actions) == 0 {
		return nil
	}

	aggregate.Next(r, PermissionRevoked, PermissionRevokedData{
		Aggregate: ref,
		Actions:   actions,
	})

	return nil
}

// IsMember returns whether the given actor is a member of this role.
func (r *Role) IsMember(actorID uuid.UUID) bool {
	for _, member := range r.members {
		if member == actorID {
			return true
		}
	}
	return false
}

// Add adds the given actors as members to the role.
func (r *Role) Add(actors ...uuid.UUID) error {
	if err := r.checkName(); err != nil {
		return err
	}
	if actors = r.nonMembers(actors); len(actors) > 0 {
		aggregate.Next(r, RoleGiven, actors)
	}
	return nil
}

func (r *Role) add(evt event.Of[[]uuid.UUID]) {
	r.members = append(r.members, evt.Data()...)
}

func (r *Role) nonMembers(actors []uuid.UUID) []uuid.UUID {
	return slice.Filter(actors, func(actorID uuid.UUID) bool {
		return !r.IsMember(actorID)
	})
}

// Remove removes the given actors as members from the role.
func (r *Role) Remove(actors ...uuid.UUID) error {
	if err := r.checkName(); err != nil {
		return err
	}
	if actors = slice.Filter(actors, r.IsMember); len(actors) > 0 {
		aggregate.Next(r, RoleRemoved, actors)
	}
	return nil
}

func (r *Role) remove(evt event.Of[[]uuid.UUID]) {
	r.members = slice.Filter(r.members, func(member uuid.UUID) bool {
		for _, actorID := range evt.Data() {
			if member == actorID {
				return false
			}
		}
		return true
	})
}
